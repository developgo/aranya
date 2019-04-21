package pod

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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

var log = logf.Log.WithName("pod")

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

			log.Info("pod object created", "infoType", "create")

			// always cache newly created pod
			mgr.podCache.Update(pod)
			// always schedule work for edge device
			mgr.podWorkQueue.MustOffer(queue.ActionCreate, pod.UID)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPod := oldObj.(*corev1.Pod).DeepCopy()
			newPod := newObj.(*corev1.Pod).DeepCopy()

			updateLog := log.WithValues("infoType", "update")

			podDeleted := !(newPod.GetDeletionTimestamp() == nil || newPod.GetDeletionTimestamp().IsZero())
			if podDeleted {
				// pod object has been deleted, but we do have cache for it,
				// which means we have that pod running on the edge device
				// delete it
				if _, ok := mgr.podCache.GetByID(newPod.UID); !ok {
					updateLog.Info("pod object already deleted, take no action")
					return
				}

				if mgr.podWorkQueue.Offer(queue.ActionDelete, newPod.UID) {
					updateLog.Info("pod object to be deleted, work scheduled")
				} else {
					updateLog.Info("pod object to be deleted, work discarded")
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

			if mgr.podWorkQueue.Offer(queue.ActionUpdate, newPod.UID) {
				log.Info("pod object updated, work scheduled")
			} else {
				log.Info("pod object updated, work discarded")
			}
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)

			deleteLog := log.WithValues("infoType", "delete")

			if _, ok := mgr.podCache.GetByID(pod.UID); !ok {
				deleteLog.Info("pod object already deleted, take no action")
				return
			}

			if mgr.podWorkQueue.Offer(queue.ActionDelete, pod.UID) {
				deleteLog.Info("pod object to be deleted, work scheduled")
			} else {
				deleteLog.Info("pod object to be deleted, work discarded")
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
		pods, err = m.GetMirrorPods()
		if err != nil {
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
							if ok := m.podWorkQueue.Offer(w.Action, w.UID); !ok {
								workLog.Info("failed to reschedule work")
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
		log.Error(err, "failed to get resolved kube pod status")
		return err
	}
	oldPod, ok := m.podCache.GetByID(status.ID)
	if !ok {
		return fmt.Errorf("failed to find pod cache by id: %v", status.ID)
	}

	apiPod, err := m.GetMirrorPod(oldPod.Namespace, oldPod.Name)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := m.DeleteDevicePod(status.ID); err != nil {
				log.Error(err, "failed to delete pod in device")
				return err
			}

			return nil
		}
		log.Error(err, "get mirror pod failed")
		return err
	}

	newPod := m.GenerateAPIPodStatus(apiPod, status)
	updatedPod, err := m.kubeClient.CoreV1().Pods(newPod.Namespace).UpdateStatus(newPod)
	if err != nil {
		log.Error(err, "failed to update kube pod status")
		return err
	}

	m.podCache.Update(updatedPod)

	return nil
}

func (m *Manager) DeleteMirrorPod(podUID types.UID) error {
	oldPod, ok := m.podCache.GetByID(podUID)
	if !ok {
		return fmt.Errorf("failed to find pod cache by id: %v", podUID)
	}

	err := m.kubeClient.CoreV1().Pods(oldPod.Namespace).Delete(oldPod.Name, &metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	m.podCache.Delete(podUID)
	return nil
}

// CreateDevicePod handle both pod resource resolution and post pod create command
func (m *Manager) CreateDevicePod(pod *corev1.Pod) error {
	createLog := log.WithValues("ns", pod.Namespace, "name", pod.Name, "uid", pod.UID)

	createLog.Info("trying to resolve containers dependencies")
	containerEnvs := make(map[string]map[string]string)
	for _, ctr := range pod.Spec.Containers {
		envs, err := resolver.ResolveEnv(m.kubeClient, pod, &ctr)
		if err != nil {
			createLog.Error(err, "failed to resolve container envs", "container", ctr.Name)
			return err
		}
		containerEnvs[ctr.Name] = envs
	}

	secrets := make([]corev1.Secret, len(pod.Spec.ImagePullSecrets))
	for i, secretRef := range pod.Spec.ImagePullSecrets {
		s, err := m.kubeClient.CoreV1().Secrets(pod.Namespace).Get(secretRef.Name, metav1.GetOptions{})
		if err != nil {
			createLog.Error(err, "failed to get image pull secret", "secret", secretRef.Name)
			return err
		}
		secrets[i] = *s
	}

	imagePullSecrets, err := resolver.ResolveImagePullSecret(pod, secrets)
	if err != nil {
		createLog.Error(err, "failed to resolve image pull secret")
		return err
	}

	volumeData, hostVolume, err := resolver.ResolveVolume(m.kubeClient, pod)
	if err != nil {
		createLog.Error(err, "failed to resolve container volumes")
		return err
	}

	// TODO: mark container status creating

	createLog.Info("trying to post pod create cmd to edge device")
	podCreateCmd := connectivity.NewPodCreateCmd(pod, imagePullSecrets, containerEnvs, volumeData, hostVolume)
	msgCh, err := m.manager.PostCmd(m.ctx, podCreateCmd)
	if err != nil {
		createLog.Error(err, "failed to post pod create command")
		return err
	}

	for msg := range msgCh {
		if err := msg.Error(); err != nil {
			// TODO: mark pod error
			createLog.Error(err, "failed to create pod in edge device")
			continue
		}

		switch podMsg := msg.GetMsg().(type) {
		case *connectivity.Msg_Pod:
			// TODO: mark pod status running
			// TODO: create net listener for published pod ports
			err = m.UpdateMirrorPod(podMsg.Pod)
			if err != nil {
				createLog.Error(err, "failed to update kube pod status")
				continue
			}
		}
	}

	return nil
}

func (m *Manager) DeleteDevicePod(podUID types.UID) (err error) {
	if _, ok := m.podCache.GetByID(podUID); !ok {
		log.Info("device pod deleted, no action")
		return nil
	}

	// TODO: get grace time
	podDeleteCmd := connectivity.NewPodDeleteCmd(string(podUID), time.Second)
	msgCh, err := m.manager.PostCmd(m.ctx, podDeleteCmd)
	if err != nil {
		log.Error(err, "failed to post pod delete command")
		return err
	}

	for msg := range msgCh {
		if err = msg.Error(); err != nil {
			log.Error(err, "failed to delete pod in device")
			continue
		}

		// TODO: delete pod objects
		// TODO: close net listener for published pod ports
		err = m.DeleteMirrorPod(podUID)
		if err != nil {
			log.Error(err, "failed to delete pod")
		}
	}

	return nil
}

func (m *Manager) SyncDevicePods() error {
	msgCh, err := m.manager.PostCmd(m.ctx, connectivity.NewPodListCmd("", "", true))
	if err != nil {
		return err
	}

	for msg := range msgCh {
		if err := msg.Error(); err != nil {
			return err
		}

		switch podMsg := msg.GetMsg().(type) {
		case *connectivity.Msg_Pod:
			err := m.UpdateMirrorPod(podMsg.Pod)
			if err != nil {
				continue
			}
		}
	}
	return nil
}
