package pod

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	kubeinformers "k8s.io/client-go/informers"
	kubeclient "k8s.io/client-go/kubernetes"
	kubelister "k8s.io/client-go/listers/core/v1"
	kubecache "k8s.io/client-go/tools/cache"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/virtualnode/connectivity"
	"arhat.dev/aranya/pkg/virtualnode/connectivity/server"
	"arhat.dev/aranya/pkg/virtualnode/pod/cache"
	"arhat.dev/aranya/pkg/virtualnode/pod/queue"
	"arhat.dev/aranya/pkg/virtualnode/resolver"
)

func NewManager(parentCtx context.Context, nodeName string, client kubeclient.Interface, manager server.Manager) *Manager {
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
		log:        logf.Log.WithName(fmt.Sprintf("%s.pod", nodeName)),
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

	podInformer.AddEventHandler(kubecache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			log := mgr.log.WithValues("type", "inform", "action", "add")

			log.Info("pod object created")
			// always cache newly created pod
			mgr.podCache.Update(pod)
			// always schedule work for edge device
			if err := mgr.podWorkQueue.Offer(queue.ActionCreate, pod.UID); err != nil {
				log.Info("work discarded", "reason", err.Error())
			} else {
				log.Info("work scheduled")
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPod := oldObj.(*corev1.Pod).DeepCopy()
			newPod := newObj.(*corev1.Pod).DeepCopy()
			log := mgr.log.WithValues("type", "inform", "action", "update")

			podDeleted := !(newPod.GetDeletionTimestamp() == nil || newPod.GetDeletionTimestamp().IsZero())
			if podDeleted {
				// pod object has been deleted, but we do have cache for it,
				// which means we have that pod running on the edge device
				// delete it
				if _, ok := mgr.podCache.GetByID(newPod.UID); !ok {
					log.Info("pod object already deleted, take no action")
					return
				}

				if err := mgr.podWorkQueue.Offer(queue.ActionDelete, newPod.UID); err != nil {
					log.Info("pod object to be deleted, work discarded", "reason", err.Error())
				} else {
					log.Info("pod object to be deleted, work scheduled")
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
				log.V(10).Info("pod spec not updated, skip")
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
			log := mgr.log.WithValues("type", "inform", "action", "delete")

			if _, ok := mgr.podCache.GetByID(pod.UID); !ok {
				log.Info("pod object already deleted, take no action")
				return
			}

			if err := mgr.podWorkQueue.Offer(queue.ActionDelete, pod.UID); err != nil {
				log.Info("work discarded", "reason", err.Error())
			} else {
				log.Info("work scheduled")
			}
		},
	})

	return mgr
}

type Manager struct {
	ctx        context.Context
	exit       context.CancelFunc
	log        logr.Logger
	manager    server.Manager
	kubeClient kubeclient.Interface
	podCache   *cache.PodCache

	podInformerFactory kubeinformers.SharedInformerFactory
	podLister          kubelister.PodLister
	podInformer        kubecache.SharedIndexInformer

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
			m.log.Error(err, "failed to get all pod assigned to this pod")
			return
		}
		for _, po := range pods {
			m.podCache.Update(po)
		}

		if ok := kubecache.WaitForCacheSync(m.ctx.Done(), m.podInformer.HasSynced); !ok {
			err = fmt.Errorf("failed to wait for caches to sync")
			m.log.Error(err, "")
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
					m.log.Info("acquiring work", "remains", m.podWorkQueue.Remains())
					podWork, shouldAcquireMore = m.podWorkQueue.Acquire()
					if !shouldAcquireMore {
						// no more work should be delivered when device is offline,
						// wait for next round
						m.log.Info("stopped acquiring work")
						return
					}

					workLog := m.log.WithValues("work", podWork.String())

					workLog.Info("work acquired, trying to deliver work")
					switch podWork.Action {
					case queue.ActionCreate:
						workLog.Info("working on creating pod in device")
						pod, found := m.podCache.GetByID(podWork.UID)
						if !found {
							workLog.Info("pod cache not found")
							continue
						}

						workLog.Info("trying to create pod in device")
						if err = m.CreateDevicePod(pod); err != nil {
							workLog.Error(err, "failed to create pod in device")
							goto handleError
						}
					case queue.ActionUpdate:
						workLog.Info("working on updating the pod in device")
						pod, found := m.podCache.GetByID(podWork.UID)
						if !found {
							workLog.Info("pod cache not found")
							continue
						}

						workLog.Info("trying to delete pod in device")
						if err = m.DeleteDevicePod(pod.UID); err != nil {
							workLog.Error(err, "failed to delete pod in device")
							goto handleError
						}

						workLog.Info("trying to delete pod in device")
						if err = m.CreateDevicePod(pod); err != nil {
							workLog.Error(err, "failed to create pod in device")
							// pod has been deleted, only need to create pod next time
							podWork.Action = queue.ActionCreate
							goto handleError
						}
					case queue.ActionDelete:
						workLog.Info("working on updating the pod in device")
						pod, found := m.podCache.GetByID(podWork.UID)
						if !found {
							workLog.Info("pod cache not found")
							continue
						}

						workLog.Info("trying to delete pod in device")
						err = m.DeleteDevicePod(pod.UID)
						if err != nil {
							workLog.Error(err, "failed to delete pod in device for update")
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

func (m *Manager) UpdateMirrorPod(pod *corev1.Pod, devicePodStatus *connectivity.PodStatus) error {
	log := m.log.WithValues("type", "cloud", "action", "update")

	if pod == nil {
		if devicePodStatus != nil {
			var ok bool

			podUID := types.UID(devicePodStatus.Uid)
			pod, ok = m.podCache.GetByID(types.UID(devicePodStatus.Uid))
			if !ok {
				return m.podWorkQueue.Offer(queue.ActionDelete, podUID)
			}
		} else {
			return fmt.Errorf("pod cache not found for device pod status")
		}
	}

	if devicePodStatus != nil {
		pod.Status.Phase, pod.Status.ContainerStatuses = resolveContainerStatus(pod, devicePodStatus)
		log.V(10).Info("resolved device container status")
	}

	log.Info("trying to update pod status")
	updatedPod, err := m.kubeClient.CoreV1().Pods(pod.Namespace).UpdateStatus(pod)
	if err != nil {
		log.Error(err, "failed to update pod status")
		return err
	}

	m.podCache.Update(updatedPod)
	return nil
}

// CreateDevicePod handle both pod resource resolution and create pod in edge device
func (m *Manager) CreateDevicePod(pod *corev1.Pod) error {
	log := m.log.WithValues("type", "device", "action", "create")

	log.Info("trying to resolve containers dependencies")
	containerEnvs := make(map[string]map[string]string)
	for _, ctr := range pod.Spec.Containers {
		envs, err := resolver.ResolveEnv(m.kubeClient, pod, &ctr)
		if err != nil {
			log.Error(err, "failed to resolve container envs", "container", ctr.Name)
			return err
		}
		containerEnvs[ctr.Name] = envs
	}

	imagePullAuthConfig, err := resolver.ResolveImagePullAuthConfig(m.kubeClient, pod)
	if err != nil {
		log.Error(err, "failed to resolve image pull secret")
		return err
	}

	volumeData, err := resolver.ResolveVolumeData(m.kubeClient, pod)
	if err != nil {
		log.Error(err, "failed to resolve container volumes")
		return err
	}

	log.Info("trying to post pod create cmd to edge device")
	podCreateOptions := translatePodCreateOptions(pod.DeepCopy(), containerEnvs, imagePullAuthConfig, volumeData)
	podCreateCmd := connectivity.NewPodCreateCmd(podCreateOptions)
	msgCh, err := m.manager.PostCmd(m.ctx, podCreateCmd)
	if err != nil {
		log.Error(err, "failed to post pod create command")
		return err
	}

	// mark pod status ContainerCreating just like what the kubelet will do
	pod.Status.Phase, pod.Status.ContainerStatuses = newContainerCreatingStatus(pod)
	log.Info("trying to update pod status to ContainerCreating")
	updatedPod, err := m.kubeClient.CoreV1().Pods(pod.Namespace).UpdateStatus(pod)
	if err != nil {
		// errored, but we should wait until work done on device
		log.Error(err, "failed to update pod status")
	} else {
		pod = updatedPod
		m.podCache.Update(pod)
	}

	for msg := range msgCh {
		if err := msg.Err(); err != nil {
			if err.Kind == connectivity.ErrAlreadyExists {
				log.Info("device pod already exits")
			} else {
				log.Error(err, "failed to create pod in edge device")
				pod.Status.Phase, pod.Status.ContainerStatuses = newContainerErrorStatus(pod)
				_ = m.UpdateMirrorPod(pod, nil)
				if err := m.podWorkQueue.Offer(queue.ActionUpdate, pod.UID); err != nil {
					log.Info("pod object to be update, work discarded", "reason", err.Error())
				} else {
					log.Info("pod object to be update, work scheduled")
				}
			}
			continue
		}

		if devicePodStatus := msg.GetPodStatus(); devicePodStatus != nil {
			_ = m.UpdateMirrorPod(pod, devicePodStatus)
		}
	}

	return nil
}

// DeleteDevicePod delete pod in edge device
func (m *Manager) DeleteDevicePod(podUID types.UID) error {
	log := m.log.WithValues("type", "device", "action", "delete")

	pod, ok := m.podCache.GetByID(podUID)
	if !ok {
		log.Info("device pod already deleted, no action")
		return nil
	}

	podDeleteCmd := connectivity.NewPodDeleteCmd(string(podUID), time.Second)
	msgCh, err := m.manager.PostCmd(m.ctx, podDeleteCmd)
	if err != nil {
		log.Error(err, "failed to post pod delete command")
		return err
	}

	for msg := range msgCh {
		if err := msg.Err(); err != nil {
			if err.Kind == connectivity.ErrNotFound {
				log.Info("device pod already deleted")
			} else {
				// TODO: should we offer another pod delete work?
				log.Error(err, "failed to delete pod in device")
				continue
			}
		}

		log.Info("trying to delete pod object immediately")
		err := m.kubeClient.CoreV1().Pods(pod.Namespace).Delete(pod.Name, metav1.NewDeleteOptions(0))
		if err != nil && !errors.IsNotFound(err) {
			log.Error(err, "failed to delete pod object")
			continue
		}

		// delete pod cache once pod object has been deleted
		m.podCache.Delete(podUID)

		// TODO: close net listener for published pod ports
	}

	return nil
}

func (m *Manager) SyncDevicePods() error {
	log := m.log.WithValues("type", "device", "action", "sync")

	log.Info("trying to sync device pods")
	msgCh, err := m.manager.PostCmd(m.ctx, connectivity.NewPodListCmd("", "", true))
	if err != nil {
		log.Error(err, "failed to post pod list cmd")
		return err
	}

	for msg := range msgCh {
		if err := msg.Err(); err != nil {
			log.Error(err, "failed to list device pods")
			continue
		}

		devicePodStatusList := msg.GetPodStatusList().GetPods()
		if devicePodStatusList == nil {
			log.Info("unexpected non pod status list")
			continue
		}

		for _, devicePodStatus := range devicePodStatusList {
			podUID := types.UID(devicePodStatus.GetUid())
			if podUID == "" {
				log.Info("device pod uid empty, discard")
				continue
			}

			pod, ok := m.podCache.GetByID(podUID)
			if !ok {
				log.Info("device pod not cached, delete")
				// no pod cache means no mirror pod for it, should be deleted
				if err := m.podWorkQueue.Offer(queue.ActionDelete, podUID); err != nil {
					log.Info("work to delete device pod discarded", "reason", err.Error())
				} else {
					log.Info("work to delete device pod scheduled")
				}
				continue
			}

			log.Info("device pod exists, update status")
			_ = m.UpdateMirrorPod(pod, devicePodStatus)
		}
	}

	return nil
}
