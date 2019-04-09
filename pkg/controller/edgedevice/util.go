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

	aranya "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/node"
)

var (
	globalHostIP   string
	globalHostname string
	mutex          sync.RWMutex
)

func (r *ReconcileEdgeDevice) getHostAddress() (ip, name string, err error) {
	ip, name = func() (ip, name string) {
		mutex.RLock()
		defer mutex.RUnlock()

		return globalHostIP, globalHostname
	}()

	if ip == "" {
		err = func() error {
			mutex.Lock()
			defer mutex.Unlock()

			currentPod := &corev1.Pod{}
			err := r.client.Get(r.ctx, types.NamespacedName{Namespace: constant.CurrentNamespace(), Name: constant.CurrentPodName()}, currentPod)
			if err != nil {
				return err
			}

			globalHostIP = currentPod.Status.HostIP
			nodeName := currentPod.Spec.NodeName

			currentNode := &corev1.Node{}
			err = r.client.Get(r.ctx, types.NamespacedName{Name: nodeName}, currentNode)
			if err != nil {
				return err
			}

			for _, addr := range currentNode.Status.Addresses {
				if addr.Type == corev1.NodeHostName {
					globalHostname = addr.Address
					break
				}
			}

			if globalHostIP == "" || globalHostname == "" {
				return fmt.Errorf("unable to find host ip and hostname")
			}

			ip = globalHostIP
			name = globalHostname
			return nil
		}()

		if err != nil {
			log.Error(errors.NewInternalError(fmt.Errorf("can't determine host ip and hostname")), "set host ip and hostname failed")
			os.Exit(1)
		}
	}

	return ip, name, nil
}

func (r *ReconcileEdgeDevice) cleanupEdgeDeviceAndVirtualNode(reqLog logr.Logger, device *aranya.EdgeDevice) (err error) {
	node.Delete(device.Name)

	needToDeleteNodeObj := true
	nodeObj := &corev1.Node{}
	err = r.client.Get(r.ctx, types.NamespacedName{Name: device.Name}, nodeObj)
	if err != nil {
		if errors.IsNotFound(err) {
			needToDeleteNodeObj = false
		} else {
			reqLog.Error(err, "get node object failed")
			return
		}
	}

	if needToDeleteNodeObj {
		err = r.client.Delete(r.ctx, nodeObj)
		if err != nil {
			reqLog.Error(err, "delete node object failed")
			return
		}
	}

	return nil
}

func getFreePort() int32 {
	port, err := freeport.GetFreePort()
	if err != nil {
		return 0
	}
	return int32(port)
}

// func (r *ReconcileEdgeDevice) runFinalizerLogic(reqLog logr.Logger, device *aranya.EdgeDevice) (deleted bool, err error) {
// 	deleted = false
//
// 	if device.DeletionTimestamp == nil || device.DeletionTimestamp.IsZero() {
// 		if !containsString(device.Finalizers, constant.FinalizerName) {
// 			device.Finalizers = append(device.Finalizers, constant.FinalizerName)
// 			reqLog.Info("update edge device finalizer")
//
// 			if err = r.client.Update(r.ctx, device); err != nil {
// 				reqLog.Error(err, "failed to update edge device finalizer")
// 				return
// 			}
// 		}
// 	} else {
// 		if containsString(device.Finalizers, constant.FinalizerName) {
// 			reqLog.Info("finalizer trying to update device")
// 			device.Finalizers = removeString(device.Finalizers, constant.FinalizerName)
// 			if err = r.client.Update(r.ctx, device); err != nil {
// 				reqLog.Error(err, "failed to update device finalizer")
// 				return
// 			}
//
// 			reqLog.Info("finalizer trying to delete related objects")
// 			if err = r.cleanup(reqLog, device); err != nil {
// 				reqLog.Error(err, "failed to delete related objects")
// 				return
// 			}
//
// 			deleted = true
// 		}
// 	}
//
// 	return
// }

// func containsString(slice []string, s string) bool {
// 	for _, item := range slice {
// 		if item == s {
// 			return true
// 		}
// 	}
// 	return false
// }
//
// func removeString(slice []string, s string) (result []string) {
// 	for _, item := range slice {
// 		if item == s {
// 			continue
// 		}
// 		result = append(result, item)
// 	}
// 	return
// }
