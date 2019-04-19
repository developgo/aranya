package pod

import (
	"context"
	"fmt"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	kubeInformers "k8s.io/client-go/informers"
	kubeClient "k8s.io/client-go/kubernetes"
	kubeListerCoreV1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/node/connectivity"
	connectivityManager "arhat.dev/aranya/pkg/node/manager"
	"arhat.dev/aranya/pkg/node/resolver"
)

var (
	log = logf.Log.WithName("aranya.node.pod")
)

func NewManager(
	ctx context.Context,
	nodeName string,
	client kubeClient.Interface,
	remoteManager connectivityManager.Interface,
) *Manager {
	podInformerFactory := kubeInformers.NewSharedInformerFactoryWithOptions(client, constant.DefaultPodReSyncInterval,
		kubeInformers.WithNamespace(corev1.NamespaceAll),
		kubeInformers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector("spec.nodeName", nodeName).String()
		}))

	return &Manager{
		ctx:                ctx,
		lister:             podInformerFactory.Core().V1().Pods().Lister(),
		podInformerFactory: podInformerFactory,
		remoteManager:      remoteManager,
		kubeClient:         client,
		podCache:           newCache(),
	}
}

type Manager struct {
	ctx                context.Context
	lister             kubeListerCoreV1.PodLister
	podInformerFactory kubeInformers.SharedInformerFactory
	kubeClient         kubeClient.Interface
	remoteManager      connectivityManager.Interface
	podCache           *Cache
}

func (m *Manager) Start() error {
	m.podInformerFactory.Core().V1().Pods().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(newObj interface{}) {
			newPod := newObj.(*corev1.Pod)

			log.Info("create pod in device")
			if err := m.CreateDevicePod(newPod); err != nil {
				log.Error(err, "failed to create pod in device", "action", "add")
				return
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPod := oldObj.(*corev1.Pod)
			newPod := newObj.(*corev1.Pod)

			podDeleted := !(newPod.DeletionTimestamp == nil || newPod.DeletionTimestamp.IsZero())
			if podDeleted {
				if err := m.DeleteDevicePod(newPod.UID); err != nil {
					log.Error(err, "failed to delete device pod")
					return
				}
			}

			// TODO: more delicate equal judgement
			if reflect.DeepEqual(oldPod.Spec, newPod.Spec) {
				// do nothing if no changes
				log.Info("skip pod update")
				return
			}

			log.Info("update pod in device")
			// delete and create
			if err := m.DeleteDevicePod(oldPod.UID); err != nil {
				log.Error(err, "failed to delete device pod", "action", "update")
				return
			}
			if err := m.CreateDevicePod(newPod); err != nil {
				log.Error(err, "failed to create device pod", "action", "update")
				return
			}
		},
		DeleteFunc: func(oldObj interface{}) {
			oldPod := oldObj.(*corev1.Pod)

			log.Info("delete pod in device")
			if err := m.DeleteDevicePod(oldPod.UID); err != nil {
				log.Error(err, "failed to delete device pod", "action", "delete")
				return
			}
		},
	})
	// start informer routine
	go m.podInformerFactory.Start(m.ctx.Done())

	// get all pods in this node
	pods, err := m.lister.List(labels.Everything())
	if err != nil {
		return err
	}

	for _, pod := range pods {
		m.podCache.Update(pod)
	}

	return nil
}

func (m *Manager) GetMirrorPods() ([]*corev1.Pod, error) {
	return m.lister.List(labels.Everything())
}

func (m *Manager) GetMirrorPod(namespace, name string) (*corev1.Pod, error) {
	return m.lister.Pods(namespace).Get(name)
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

func (m *Manager) CreateDevicePod(pod *corev1.Pod) error {
	m.podCache.Update(pod)

	containerEnvs := make(map[string]map[string]string)
	for _, ctr := range pod.Spec.Containers {
		envs, err := resolver.ResolveEnv(m.kubeClient, pod, &ctr)
		if err != nil {
			log.Error(err, "failed to resolve pod envs")
			return err
		}
		containerEnvs[ctr.Name] = envs
	}

	secrets := make([]corev1.Secret, len(pod.Spec.ImagePullSecrets))
	for i, secretRef := range pod.Spec.ImagePullSecrets {
		s, err := m.kubeClient.CoreV1().Secrets(pod.Namespace).Get(secretRef.Name, metav1.GetOptions{})
		if err != nil {
			log.Error(err, "failed to get image pull secret")
			return err
		}
		secrets[i] = *s
	}

	imagePullSecrets, err := resolver.ResolveImagePullSecret(pod, secrets)
	if err != nil {
		log.Error(err, "failed to resolve image pull secret")
		return err
	}

	volumeData, hostVolume, err := resolver.ResolveVolume(m.kubeClient, pod)
	if err != nil {
		log.Error(err, "failed to resolve volume")
		return err
	}

	podCreateCmd := connectivity.NewPodCreateCmd(pod, imagePullSecrets, containerEnvs, volumeData, hostVolume)
	msgCh, err := m.remoteManager.PostCmd(m.ctx, podCreateCmd)
	if err != nil {
		log.Error(err, "failed to post pod create command")
		return err
	}

	for msg := range msgCh {
		if err := msg.Error(); err != nil {
			log.Error(err, "failed to create pod in device")
			continue
		}

		switch podMsg := msg.GetMsg().(type) {
		case *connectivity.Msg_Pod:
			err = m.UpdateMirrorPod(podMsg.Pod)
			if err != nil {
				log.Error(err, "failed to update kube pod status")
				continue
			}
		}
	}

	return nil
}

func (m *Manager) DeleteDevicePod(podUID types.UID) (err error) {
	podDeleteCmd := connectivity.NewPodDeleteCmd(string(podUID), time.Minute)
	msgCh, err := m.remoteManager.PostCmd(m.ctx, podDeleteCmd)
	if err != nil {
		log.Error(err, "failed to post pod delete command")
	}

	for msg := range msgCh {
		if err = msg.Error(); err != nil {
			log.Error(err, "failed to delete pod in device")
			continue
		}

		switch msg.GetMsg().(type) {
		case *connectivity.Msg_Pod:
			err = m.DeleteMirrorPod(podUID)
			if err != nil {
				log.Error(err, "failed to delete pod")
			}
		}
	}

	return nil
}

func (m *Manager) SyncDevicePods() error {
	msgCh, err := m.remoteManager.PostCmd(m.ctx, connectivity.NewPodListCmd("", "", true))
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
