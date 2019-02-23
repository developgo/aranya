package edgedevice

import (
	"fmt"
	"os"
	"sync"

	"github.com/go-logr/logr"
	"github.com/phayes/freeport"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	aranyav1alpha1 "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/node"
)

var (
	hostIP string
	mutex  sync.RWMutex
)

func (r *ReconcileEdgeDevice) runFinalizerLogic(reqLog logr.Logger, device *aranyav1alpha1.EdgeDevice) (err error) {
	if device.DeletionTimestamp == nil || device.DeletionTimestamp.IsZero() {
		if !containsString(device.Finalizers, constant.FinalizerName) {
			device.Finalizers = append(device.Finalizers, constant.FinalizerName)
			reqLog.Info("update edge device finalizer")
			if err = r.client.Update(r.ctx, device); err != nil {
				reqLog.Error(err, "update edge device finalizer failed")
				return
			}
		}
	} else {
		if containsString(device.Finalizers, constant.FinalizerName) {
			reqLog.Info("finalizer trying to delete related objects")
			if err = r.deleteRelatedResourceObjects(device); err != nil {
				reqLog.Error(err, "finalizer delete related objects failed")
				return
			}

			reqLog.Info("finalizer trying to update device")
			device.Finalizers = removeString(device.Finalizers, constant.FinalizerName)
			if err = r.client.Update(r.ctx, device); err != nil {
				reqLog.Error(err, "finalizer update device failed")
				return
			}
		}
	}

	return
}

func (r *ReconcileEdgeDevice) getHostIP() (string, error) {
	ip := func() string {
		mutex.RLock()
		defer mutex.RUnlock()

		return hostIP
	}()

	if ip == "" {
		mutex.Lock()
		defer mutex.Unlock()

		currentPod := &corev1.Pod{}
		err := r.client.Get(r.ctx, types.NamespacedName{Namespace: constant.CurrentNamespace(), Name: constant.CurrentPodName()}, currentPod)
		if err != nil {
			return "", err
		}

		hostIP = currentPod.Status.HostIP
		ip = hostIP
	}

	if ip == "" {
		log.Error(errors.NewInternalError(fmt.Errorf("can't determine host ip")), "can't determine host ip")
		os.Exit(1)
	}

	return ip, nil
}

func (r *ReconcileEdgeDevice) deleteRelatedResourceObjects(device *aranyav1alpha1.EdgeDevice) (err error) {
	nodeObj := &corev1.Node{}
	err = r.client.Get(r.ctx, types.NamespacedName{Name: device.Name}, nodeObj)
	if err != nil {
		return err
	}

	err = r.client.Delete(r.ctx, nodeObj)
	if err != nil {
		return err
	}

	switch device.Spec.Connectivity.Method {
	case aranyav1alpha1.DeviceConnectViaGRPC:
		svcObj := &corev1.Service{}
		err = r.client.Get(r.ctx, types.NamespacedName{Name: device.Name, Namespace: device.Namespace}, svcObj)
		if err != nil {
			return err
		}

		err = r.client.Delete(r.ctx, svcObj)
		if err != nil {
			return err
		}
	}

	node.DeleteRunningServer(device.Name)

	return nil
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

func getFreePort() int32 {
	port, err := freeport.GetFreePort()
	if err != nil {
		return 0
	}
	return int32(port)
}
