package pod

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	kubeinformers "k8s.io/client-go/informers"
	kubeclient "k8s.io/client-go/kubernetes"
	kubelister "k8s.io/client-go/listers/core/v1"
	k8scache "k8s.io/client-go/tools/cache"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/virtualnode/connectivity"
	"arhat.dev/aranya/pkg/virtualnode/manager"
	"arhat.dev/aranya/pkg/virtualnode/pod/cache"
	"arhat.dev/aranya/pkg/virtualnode/pod/queue"
	"arhat.dev/aranya/pkg/virtualnode/resolver"
)

var (
	log               = logf.Log.WithName("pod")
	httpLog           = log.WithValues("httpAction", "request")
	httpStreamLog     = log.WithValues("httpAction", "stream")
	informerCreateLog = log.WithValues("informerAction", "create")
	informerUpdateLog = log.WithValues("informerAction", "update")
	informerDeleteLog = log.WithValues("informerAction", "delete")
	mirrorUpdateLog   = log.WithValues("mirrorAction", "update")
	mirrorDeleteLog   = log.WithValues("mirrorAction", "delete")
	deviceCreateLog   = log.WithValues("deviceAction", "create")
	deviceDeleteLog   = log.WithValues("deviceAction", "delete")
	deviceSyncLog     = log.WithValues("deviceAction", "sync")
)

func NewManager(parentCtx context.Context, nodeName string, client kubeclient.Interface, manager manager.Manager) *Manager {
	podInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(
		client, constant.DefaultPodReSyncInterval,
		// watch all pods scheduled to the node
		kubeinformers.WithNamespace(corev1.NamespaceAll),
		kubeinformers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector("spec.nodeName", nodeName).String()
		}),
	)

	podInformer := podInformerFactory.Core().V1().Pods().Informer()
	ctx, exit := context.WithCancel(parentCtx)
	mgr := &Manager{
		ctx:        ctx,
		exit:       exit,
		manager:    manager,
		kubeClient: client,
		podCache:   cache.NewPodCache(),

		podInformerFactory: podInformerFactory,
		podLister:          podInformerFactory.Core().V1().Pods().Lister(),
		podInformer:        podInformer,

		podWorkQueue: queue.NewWorkQueue(),
	}

	podInformer.AddEventHandler(k8scache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)

			informerCreateLog.Info("pod object created")
			// always cache newly created pod
			mgr.podCache.Update(pod)
			// always schedule work for edge device
			if err := mgr.podWorkQueue.Offer(queue.ActionCreate, pod.UID); err != nil {
				informerCreateLog.Info("work discarded", "reason", err.Error())
			} else {
				informerCreateLog.Info("work scheduled")
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPod := oldObj.(*corev1.Pod).DeepCopy()
			newPod := newObj.(*corev1.Pod).DeepCopy()

			podDeleted := !(newPod.GetDeletionTimestamp() == nil || newPod.GetDeletionTimestamp().IsZero())
			if podDeleted {
				// pod object has been deleted, but we do have cache for it,
				// which means we have that pod running on the edge device
				// delete it
				if _, ok := mgr.podCache.GetByID(newPod.UID); !ok {
					informerUpdateLog.Info("pod object already deleted, take no action")
					return
				}

				if err := mgr.podWorkQueue.Offer(queue.ActionDelete, newPod.UID); err != nil {
					informerUpdateLog.Info("pod object to be deleted, work discarded", "reason", err.Error())
				} else {
					informerUpdateLog.Info("pod object to be deleted, work scheduled")
				}

				// we don't want to cache a deleted pod in any case
				return
			}

			// when pod object has been updated (and not deleted),
			// we cache the new pod for possible future use
			mgr.podCache.Update(newPod)

			// pod need to be updated on device only when its spec has been changed
			// TODO: evaluate more delicate check
			if reflect.DeepEqual(oldPod.Spec, newPod.Spec) {
				log.Info("pod object not updated, skip")
				return
			}

			if err := mgr.podWorkQueue.Offer(queue.ActionUpdate, newPod.UID); err != nil {
				log.Info("pod object updated, work discarded", "reason", err.Error())
			} else {
				log.Info("pod object updated, work scheduled")
			}
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)

			if _, ok := mgr.podCache.GetByID(pod.UID); !ok {
				informerDeleteLog.Info("pod object already deleted, take no action")
				return
			}

			if err := mgr.podWorkQueue.Offer(queue.ActionDelete, pod.UID); err != nil {
				informerDeleteLog.Info("work discarded", "reason", err.Error())
			} else {
				informerDeleteLog.Info("work scheduled")
			}
		},
	})

	return mgr
}

type Manager struct {
	ctx        context.Context
	exit       context.CancelFunc
	manager    manager.Manager
	kubeClient kubeclient.Interface
	podCache   *cache.PodCache

	podInformerFactory kubeinformers.SharedInformerFactory
	podLister          kubelister.PodLister
	podInformer        k8scache.SharedIndexInformer

	podWorkQueue *queue.WorkQueue
	once         sync.Once
}

func (m *Manager) Start() (err error) {
	err = fmt.Errorf("manager started once, do not start again")
	m.once.Do(func() {
		// start informer routine
		go m.podInformerFactory.Start(m.ctx.Done())

		// get all pods assigned to this node and build the pod cache
		var pods []*corev1.Pod
		if pods, err = m.GetMirrorPods(); err != nil {
			log.Error(err, "failed to get all pod assigned to this pod")
			return
		}
		for _, po := range pods {
			m.podCache.Update(po)
		}

		if ok := k8scache.WaitForCacheSync(m.ctx.Done(), m.podInformer.HasSynced); !ok {
			err = fmt.Errorf("failed to wait for caches to sync")
			log.Error(err, "")
			return
		}

		for !m.exiting() {
			// prevent work to be delivered when device is offline
			m.podWorkQueue.Stop()

			select {
			case <-m.manager.Connected():
				// we are good to go
			case <-m.ctx.Done():
				return
			}

			// must called before any acquire call
			m.podWorkQueue.Start()
			go func() {
				var (
					err               error
					podWork           queue.Work
					shouldAcquireMore bool
				)

				for {
					log.Info("acquiring work", "remains", m.podWorkQueue.Remains())
					podWork, shouldAcquireMore = m.podWorkQueue.Acquire()
					if !shouldAcquireMore {
						// no more work should be delivered when device is offline,
						// wait for next round
						log.Info("stopped acquiring work")
						return
					}

					workLog := log.WithValues("work", podWork.String())

					workLog.Info("work acquired, trying to deliver work")
					switch podWork.Action {
					case queue.ActionCreate:
						workLog.Info("working on creating pod in device")
						pod, found := m.podCache.GetByID(podWork.UID)
						if !found {
							log.Info("pod cache not found")
							continue
						}

						workLog.Info("trying to create pod in device")
						if err = m.CreateDevicePod(pod); err != nil {
							log.Error(err, "failed to create pod in device")
							goto handleError
						}
					case queue.ActionUpdate:
						log.Info("working on updating the pod in device")
						pod, found := m.podCache.GetByID(podWork.UID)
						if !found {
							log.Info("pod cache not found")
							continue
						}

						log.Info("trying to delete pod in device")
						if err = m.DeleteDevicePod(pod.UID); err != nil {
							log.Error(err, "failed to delete pod in device")
							goto handleError
						}

						workLog.Info("trying to delete pod in device")
						if err = m.CreateDevicePod(pod); err != nil {
							log.Error(err, "failed to create pod in device")
							// pod has been deleted, only need to create pod next time
							podWork.Action = queue.ActionCreate
							goto handleError
						}
					case queue.ActionDelete:
						log.Info("working on updating the pod in device")
						pod, found := m.podCache.GetByID(podWork.UID)
						if !found {
							log.Info("pod cache not found")
							continue
						}

						log.Info("trying to delete pod in device")
						err = m.DeleteDevicePod(pod.UID)
						if err != nil {
							log.Error(err, "failed to delete pod in device for update")
							goto handleError
						}
					default:
						// invalid work, discard
						continue
					}
				handleError:
					if err != nil {
						// requeue work when error happened
						workLog.Info("exception happened for work, reschedule same work in future")
						go func(w queue.Work) {
							time.Sleep(time.Second)
							if err := m.podWorkQueue.Offer(w.Action, w.UID); err != nil {
								workLog.Info("failed to reschedule work", "reason", err.Error())
							} else {
								workLog.Info("work rescheduled")
							}
						}(podWork)
					}
				}
			}()

			// wait until device disconnected
			select {
			case <-m.manager.Disconnected():
				// stop work queue acquire, the work queue will keep collecting work items
				m.podWorkQueue.Stop()
				continue
			case <-m.ctx.Done():
				m.podWorkQueue.Stop()
				return
			}
		}
	})

	return
}

func (m *Manager) Stop() {
	m.exit()
}

func (m *Manager) exiting() bool {
	select {
	case <-m.ctx.Done():
		return true
	default:
		return false
	}
}

func (m *Manager) GetMirrorPods() ([]*corev1.Pod, error) {
	return m.podLister.List(labels.Everything())
}

func (m *Manager) GetMirrorPod(namespace, name string) (*corev1.Pod, error) {
	return m.podLister.Pods(namespace).Get(name)
}

func (m *Manager) UpdateMirrorPod(devicePod *connectivity.Pod) error {
	status, err := devicePod.GetResolvedKubePodStatus()
	if err != nil {
		mirrorUpdateLog.Error(err, "failed to get resolved kube pod status")
		return err
	}

	oldPod, ok := m.podCache.GetByID(status.ID)
	if !ok {
		mirrorUpdateLog.Info("failed to find pod cache", "podUID", status.ID)
		return nil
	}

	newPod := m.GenerateAPIPodStatus(oldPod, status)
	updatedPod, err := m.kubeClient.CoreV1().Pods(oldPod.Namespace).UpdateStatus(newPod)
	if err != nil {
		mirrorUpdateLog.Error(err, "failed to update pod status")
		return err
	}

	m.podCache.Update(updatedPod)
	return nil
}

// DeleteMirrorPod delete pod object immediately and clear pod cache once delete succeeded
func (m *Manager) DeleteMirrorPod(podUID types.UID) error {
	pod, ok := m.podCache.GetByID(podUID)
	if !ok {
		mirrorDeleteLog.Info("failed to find pod cache", "podUID", podUID)
		return nil
	}

	err := m.kubeClient.CoreV1().Pods(pod.Namespace).Delete(pod.Name, metav1.NewDeleteOptions(0))
	if err != nil {
		mirrorDeleteLog.Error(err, "failed to delete pod object")
		return err
	}

	// delete pod cache once pod object has been deleted
	m.podCache.Delete(podUID)
	return nil
}

// CreateDevicePod handle both pod resource resolution and create pod in edge device
func (m *Manager) CreateDevicePod(pod *corev1.Pod) error {
	deviceCreateLog.Info("trying to resolve containers dependencies")
	containerEnvs := make(map[string]map[string]string)
	for _, ctr := range pod.Spec.Containers {
		envs, err := resolver.ResolveEnv(m.kubeClient, pod, &ctr)
		if err != nil {
			deviceCreateLog.Error(err, "failed to resolve container envs", "container", ctr.Name)
			return err
		}
		containerEnvs[ctr.Name] = envs
	}

	secrets := make([]corev1.Secret, len(pod.Spec.ImagePullSecrets))
	for i, secretRef := range pod.Spec.ImagePullSecrets {
		s, err := m.kubeClient.CoreV1().Secrets(pod.Namespace).Get(secretRef.Name, metav1.GetOptions{})
		if err != nil {
			deviceCreateLog.Error(err, "failed to get image pull secret", "secret", secretRef.Name)
			return err
		}
		secrets[i] = *s
	}

	imagePullSecrets, err := resolver.ResolveImagePullSecret(pod, secrets)
	if err != nil {
		deviceCreateLog.Error(err, "failed to resolve image pull secret")
		return err
	}

	volumeData, hostVolume, err := resolver.ResolveVolume(m.kubeClient, pod)
	if err != nil {
		deviceCreateLog.Error(err, "failed to resolve container volumes")
		return err
	}

	// TODO: mark container status creating

	deviceCreateLog.Info("trying to post pod create cmd to edge device")
	podCreateCmd := connectivity.NewPodCreateCmd(pod, imagePullSecrets, containerEnvs, volumeData, hostVolume)
	msgCh, err := m.manager.PostCmd(m.ctx, podCreateCmd)
	if err != nil {
		deviceCreateLog.Error(err, "failed to post pod create command")
		return err
	}

	for msg := range msgCh {
		if err := msg.Error(); err != nil {
			// TODO: mark pod error
			deviceCreateLog.Error(err, "failed to create pod in edge device")
			continue
		}

		switch podMsg := msg.GetMsg().(type) {
		case *connectivity.Msg_Pod:
			// TODO: mark pod status running
			// TODO: create net listener for published pod ports
			err = m.UpdateMirrorPod(podMsg.Pod)
			if err != nil {
				deviceCreateLog.Error(err, "failed to update kube pod status")
				continue
			}
		}
	}

	return nil
}

// DeleteDevicePod delete pod in edge device
func (m *Manager) DeleteDevicePod(podUID types.UID) (err error) {
	if _, ok := m.podCache.GetByID(podUID); !ok {
		deviceDeleteLog.Info("device pod already deleted, no action")
		return nil
	}

	// TODO: get grace time
	podDeleteCmd := connectivity.NewPodDeleteCmd(string(podUID), time.Second)
	msgCh, err := m.manager.PostCmd(m.ctx, podDeleteCmd)
	if err != nil {
		deviceDeleteLog.Error(err, "failed to post pod delete command")
		return err
	}

	for msg := range msgCh {
		if err = msg.Error(); err != nil {
			deviceDeleteLog.Error(err, "failed to delete pod in device")
			continue
		}

		// TODO: delete pod objects
		// TODO: close net listener for published pod ports
		if err = m.DeleteMirrorPod(podUID); err != nil {
			deviceDeleteLog.Error(err, "failed to delete pod object")
		}
	}

	return nil
}

func (m *Manager) SyncDevicePods() (err error) {
	var msgCh <-chan *connectivity.Msg

	msgCh, err = m.manager.PostCmd(m.ctx, connectivity.NewPodListCmd("", "", true))
	if err != nil {
		deviceSyncLog.Error(err, "failed to post pod list cmd")
		return err
	}

	for msg := range msgCh {
		if err = msg.Error(); err != nil {
			deviceSyncLog.Error(err, "failed to list device pods")
			continue
		}

		switch podMsg := msg.GetMsg().(type) {
		case *connectivity.Msg_Pod:
			podUID := types.UID(podMsg.Pod.GetUid())
			if podUID == "" {
				deviceSyncLog.Info("device pod uid empty, discard")
				continue
			}

			if _, ok := m.podCache.GetByID(podUID); !ok {
				deviceSyncLog.Info("device pod not cached, delete")
				// no pod cache means no mirror pod for it, should be deleted
				if err := m.podWorkQueue.Offer(queue.ActionDelete, podUID); err != nil {
					log.Info("work to delete device pod discarded", "reason", err.Error())
				} else {
					log.Info("work to delete device pod scheduled")
				}
				continue
			}

			deviceSyncLog.Info("device pod exists, update status")
			if err = m.UpdateMirrorPod(podMsg.Pod); err != nil {
				deviceSyncLog.Error(err, "failed to update pod status")
				continue
			}
		}
	}

	return nil
}
