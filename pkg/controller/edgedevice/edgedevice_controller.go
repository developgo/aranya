/*
Copyright 2019 The arhat.dev Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package edgedevice

import (
	"context"
	"crypto/tls"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	"github.com/operator-framework/operator-sdk/pkg/log/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	controllermanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	aranya "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/connectivity/server"
	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/virtualnode"
)

var (
	once = &sync.Once{}
)

var log = logf.Log.WithName("aranya")

func init() {
	logf.SetLogger(zap.Logger())
}

// AddToManager creates a new EdgeDevice Controller and adds it to the manager. The manager will set fields on the Controller
// and Start it when the manager is Started.
func AddToManager(mgr controllermanager.Manager, config *virtualnode.Config) error {
	return addToManager(mgr, &ReconcileEdgeDevice{
		client:            mgr.GetClient(),
		scheme:            mgr.GetScheme(),
		config:            mgr.GetConfig(),
		ctx:               context.Background(),
		kubeClient:        kubeclient.NewForConfigOrDie(mgr.GetConfig()),
		VirtualNodeConfig: config,
	})
}

// addToManager adds a new Controller to mgr with r as the reconcile.Reconciler
func addToManager(mgr controllermanager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("aranya", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch changes to EdgeDevice resources
	if err = c.Watch(&source.Kind{Type: &aranya.EdgeDevice{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	// Watch node objects created by EdgeDevice object
	if err = c.Watch(&source.Kind{Type: &corev1.Node{}}, &handler.EnqueueRequestForOwner{
		IsController: true, OwnerType: &aranya.EdgeDevice{},
	}); err != nil {
		return err
	}

	// watch service objects created by EdgeDevice object
	svcMapper := &ServiceMapper{client: mgr.GetClient()}
	if err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: svcMapper,
	}); err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileEdgeDevice{}

// ReconcileEdgeDevice reconciles a EdgeDevice object
type ReconcileEdgeDevice struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client     client.Client
	kubeClient kubeclient.Interface
	scheme     *runtime.Scheme
	config     *rest.Config
	ctx        context.Context

	VirtualNodeConfig *virtualnode.Config
}

// Reconcile reads that state of the cluster for a EdgeDevice object and makes changes based on the state read
// and what is in the EdgeDevice.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileEdgeDevice) Reconcile(request reconcile.Request) (result reconcile.Result, err error) {
	once.Do(func() {
		log.Info("initialize edge devices")
		// get all edge devices in this namespace (only once)
		deviceList := &aranya.EdgeDeviceList{}
		err = r.client.List(r.ctx, &client.ListOptions{Namespace: constant.CurrentNamespace()}, deviceList)
		if err != nil {
			log.Error(err, "failed to list edge devices")
			return
		}

		for _, device := range deviceList.Items {
			if err := r.doReconcileEdgeDevice(log, device.Namespace, device.Name); err != nil {
				log.Error(err, "reconcile edge device failed", "device", device.Name)
			}
		}
	})
	if err != nil {
		log.Error(err, "failed to initialize edge devices")
		return reconcile.Result{}, err
	}

	reqLog := log.WithValues("name", request.Name)
	if request.Namespace == corev1.NamespaceAll {
		// check this node belongs to this namespace
		nodeObj := &corev1.Node{}
		err = r.client.Get(r.ctx, types.NamespacedName{Namespace: corev1.NamespaceAll, Name: request.Name}, nodeObj)
		if err != nil {
			if !errors.IsNotFound(err) {
				return reconcile.Result{}, err
			}

			// we're not sure which namespace this node belongs to
			// reconcile the edge device to make sure we are in consistent state
			return reconcile.Result{}, r.doReconcileEdgeDevice(reqLog, constant.CurrentNamespace(), request.Name)
		}

		belongsToThisNamespace := false
		for _, t := range nodeObj.Spec.Taints {
			if t.Key == constant.TaintKeyNamespace && t.Value == constant.CurrentNamespace() {
				belongsToThisNamespace = true
				break
			}
		}

		if !belongsToThisNamespace {
			return reconcile.Result{}, nil
		}

		// reconcile node only
		return reconcile.Result{}, r.doReconcileVirtualNode(reqLog, constant.CurrentNamespace(), request.Name, nil)
	}

	reqLog = reqLog.WithValues("ns", request.Namespace)
	return reconcile.Result{}, r.doReconcileEdgeDevice(reqLog, request.Namespace, request.Name)
}

func (r *ReconcileEdgeDevice) doReconcileEdgeDevice(reqLog logr.Logger, namespace, name string) (err error) {
	var (
		deviceObj     = &aranya.EdgeDevice{}
		deviceDeleted = false
	)

	// get the edge device instance
	err = r.client.Get(r.ctx, types.NamespacedName{Namespace: namespace, Name: name}, deviceObj)
	if err != nil {
		if !errors.IsNotFound(err) {
			reqLog.Error(err, "failed to get edge device")
			return err
		}

		// device not found, could be deleted by user
		deviceDeleted = true
	} else {
		// check if device has been deleted
		deviceDeleted = !(deviceObj.DeletionTimestamp == nil || deviceObj.DeletionTimestamp.IsZero())
	}

	// edge device need to be deleted, cleanup
	if deviceDeleted {
		reqLog.Info("edge device deleted, cleaning up virtual node")
		err = r.cleanupVirtualNode(reqLog, namespace, name)
		if err != nil {
			reqLog.Error(err, "failed to cleanup virtual node")
			return err
		}

		return nil
	}

	//
	// edge device exists, check its related virtual node
	//
	err = r.doReconcileVirtualNode(reqLog, namespace, name, deviceObj)
	if err != nil {
		return err
	}

	return nil
}

func (r *ReconcileEdgeDevice) doReconcileVirtualNode(reqLog logr.Logger, namespace, name string, deviceObj *aranya.EdgeDevice) (err error) {
	var (
		nodeObj      = &corev1.Node{}
		svcObj       = &corev1.Service{}
		creationOpts = &virtualnode.CreationOptions{KubeClient: r.kubeClient}
		virtualNode  *virtualnode.VirtualNode

		needToCheckDeviceObject bool
		needToCreateNodeObject  bool
		needToCreateVirtualNode bool
		ok                      bool
	)

	err = r.client.Get(r.ctx, types.NamespacedName{Namespace: corev1.NamespaceAll, Name: name}, nodeObj)
	if err != nil {
		if !errors.IsNotFound(err) {
			reqLog.Error(err, "failed to get node object")
			return err
		}

		// since the node object not found (has been deleted),
		// delete the related virtual node
		reqLog.Info("node object not found, destroying virtual node")
		virtualnode.Delete(name)

		needToCheckDeviceObject = true
		needToCreateNodeObject = true
	} else {
		nodeDeleted := !(nodeObj.DeletionTimestamp == nil || nodeObj.DeletionTimestamp.IsZero())
		if nodeDeleted {
			// node to be deleted, delete the related virtual node
			reqLog.Info("node object deleted, destroying virtual node")
			virtualnode.Delete(name)

			needToCheckDeviceObject = true
			needToCreateNodeObject = true
		} else {
			// node presents and not deleted, virtual node MUST exist
			// (or we need to delete the all related objects)
			virtualNode, ok = virtualnode.Get(name)
			if !ok {
				if err = r.cleanupVirtualNode(reqLog, namespace, name); err != nil {
					return err
				}

				return fmt.Errorf("unexpected virtual node not present")
			} else {
				// get previous create options
				oldOpts := virtualNode.CreationOptions()

				creationOpts.KubeletServerListener = oldOpts.KubeletServerListener
				creationOpts.GRPCServerListener = oldOpts.GRPCServerListener
				creationOpts.Manager = oldOpts.Manager
			}
		}
	}

	if needToCheckDeviceObject {
		reqLog.Info("trying to get edge device for node check")

		deviceObj = &aranya.EdgeDevice{}
		err = r.client.Get(r.ctx, types.NamespacedName{Namespace: namespace, Name: name}, deviceObj)
		if err != nil {
			if errors.IsNotFound(err) {
				reqLog.Info("edge device not found, node can be deleted")
				return nil
			}

			reqLog.Error(err, "failed to get edge device for node")
			return err
		} else {
			deviceDeleted := !(deviceObj.GetDeletionTimestamp() == nil || deviceObj.GetDeletionTimestamp().IsZero())
			if deviceDeleted {
				reqLog.Info("edge device deleted, node can be deleted")
				return nil
			}
		}
	}

	// device nil means this function was called for the node only or
	// the device has been deleted, no more action required
	if deviceObj == nil {
		reqLog.V(10).Info("finished reconcile for node object")
		return nil
	}

	// here, device presents and not deleted
	// everything related expected to exist
	creationOpts.Config = r.VirtualNodeConfig.OverrideWith(deviceObj.Spec.Connectivity.Timers)

	if needToCreateNodeObject {
		// new node and new virtual node
		reqLog.Info("trying to create node object")
		creationOpts.NodeObject, creationOpts.KubeletServerListener, err = r.createNodeObjectForDevice(deviceObj)
		if err != nil {
			reqLog.Error(err, "failed to create node object")
			return err
		}

		needToCreateVirtualNode = true

		defer func() {
			if err != nil {
				_ = creationOpts.KubeletServerListener.Close()

				reqLog.Info("cleanup virtual node due to error")
				if err := r.cleanupVirtualNode(reqLog, namespace, name); err != nil {
					reqLog.Error(err, "failed to cleanup virtual node")
					return
				}
			}
		}()
	}

	// check device connectivity, check service object if grpc is used
	switch deviceObj.Spec.Connectivity.Method {
	case aranya.GRPC:
		reqLog.Info("trying to get svc object")

		svcDeleted := false
		svcNamespacedName := types.NamespacedName{Namespace: namespace, Name: name}
		err = r.client.Get(r.ctx, svcNamespacedName, svcObj)
		if err != nil {
			if !errors.IsNotFound(err) {
				reqLog.Error(err, "failed to get svc object")
				return err
			}

			svcDeleted = true
		} else {
			// service object exists, but to be deleted,
			svcDeleted = !(svcObj.DeletionTimestamp == nil || svcObj.DeletionTimestamp.IsZero())
		}

		if svcDeleted {
			svcObj = nil
		}

		// service object doesn't exists,
		// whether not created with virtual node or has been deleted
		if needToCreateVirtualNode {
			// need to create service object and grpc server, then we need a new grpc listener
			reqLog.Info("trying to create grpc svc for device")

			grpcConfig := deviceObj.Spec.Connectivity.GRPCConfig
			svcObj, creationOpts.GRPCServerListener, err = r.createGRPCSvcObjectForDevice(deviceObj, svcObj)
			if err != nil {
				return err
			}

			// close newly created grpc listener on error
			defer func() {
				if err != nil {
					_ = creationOpts.GRPCServerListener.Close()
				}
			}()

			var grpcSrvOptions []grpc.ServerOption
			if tlsRef := grpcConfig.TLSSecretRef; tlsRef != nil && tlsRef.Name != "" {
				var grpcServerCert *tls.Certificate

				reqLog.Info("trying to get grpc server tls secret")
				grpcServerCert, err = r.GetCertFromSecret(namespace, tlsRef.Name)
				if err != nil {
					reqLog.Error(err, "failed to get grpc server tls secret")
					return err
				}
				grpcSrvOptions = append(grpcSrvOptions, grpc.Creds(credentials.NewServerTLSFromCert(grpcServerCert)))
			}

			creationOpts.Manager = server.NewGRPCManager(grpc.NewServer(grpcSrvOptions...), creationOpts.GRPCServerListener, &creationOpts.Config.Connectivity)
		} else {
			// service object deleted (most likely deleted by user)
			// create service object according to existing grpc listener
			if creationOpts.GRPCServerListener != nil {
				// existing virtual node work as a grpc manager
				// we just need to create a service object for it

				// NOTICE: any grpc config update will not be applied
				// TODO: should we support grpc config change?
				var port int32
				port, err = GetListenPort(creationOpts.GRPCServerListener.Addr().String())
				if err != nil {
					reqLog.Error(err, "failed to get grpc listening port")
					return err
				}

				if !svcDeleted {
					// check if svc has right port
					oldPort := int32(0)
					if len(svcObj.Spec.Ports) > 0 {
						oldPort = svcObj.Spec.Ports[0].TargetPort.IntVal
					}

					if oldPort == port {
						// current svc has updated port value, finish reconcile
						reqLog.Info("current svc is update to date")
						return nil
					}

					reqLog.Info("trying to delete outdated svc object")
					err = r.client.Delete(r.ctx, svcObj, client.GracePeriodSeconds(0))
					if err != nil {
						reqLog.Error(err, "failed to delete svc object")
						return err
					}
				}

				// current svc is not up to date, need to delete and create new one
				reqLog.Info("trying to set controller reference to svc object")
				newSvcObj := newServiceForEdgeDevice(deviceObj, port)
				err = controllerutil.SetControllerReference(deviceObj, newSvcObj, r.scheme)
				if err != nil {
					reqLog.Error(err, "failed to set controller reference to svc object")
					return err
				}

				reqLog.Info("trying to create updated svc object")
				err = r.client.Create(r.ctx, newSvcObj)
				if err != nil {
					reqLog.Error(err, "failed to create updated svc object")
					return err
				}

				svcObj = newSvcObj
			} else {
				// existing virtual node doesn't work as grpc manager
				// changes happen in EdgeDevice's spec
				// TODO: should we support connectivity method change?
				reqLog.Info("connectivity method change not supported")
			}
		}
	case aranya.MQTT:
		if !needToCreateVirtualNode {
			// nothing to do since mqtt doesn't require any service object
			// TODO: should we support connectivity method change?
			break
		}

		mqttConfig := deviceObj.Spec.Connectivity.MQTTConfig
		var cert *tls.Certificate
		if tlsRef := mqttConfig.TLSSecretRef; tlsRef != nil && tlsRef.Name != "" {
			reqLog.Info("trying to get mqtt client tls secret")
			cert, err = r.GetCertFromSecret(namespace, tlsRef.Name)
			if err != nil {
				reqLog.Error(err, "failed to get mqtt client tls secret")
				return err
			}
		}

		reqLog.Info("trying to create mqtt connectivity manager")
		creationOpts.Manager, err = server.NewMQTTManager(mqttConfig, cert, &creationOpts.Config.Connectivity)
		if err != nil {
			reqLog.Error(err, "failed to create mqtt connectivity manager")
			return err
		}
	default:
		return fmt.Errorf("unsupported connectivity method")
	}

	// create virtual node if required
	if needToCreateVirtualNode {
		reqLog.Info("creating virtual node", "options", creationOpts)
		virtualNode, err = virtualnode.CreateVirtualNode(r.ctx, creationOpts)
		if err != nil {
			reqLog.Error(err, "failed to create virtual node")
			return err
		}

		reqLog.Info("trying to start virtual node")
		if err = virtualNode.Start(); err != nil {
			reqLog.Error(err, "failed to start virtual node")
			return err
		}
	}

	return nil
}

func (r *ReconcileEdgeDevice) GetCertFromSecret(namespace, name string) (*tls.Certificate, error) {
	tlsSecret := &corev1.Secret{}
	err := r.client.Get(r.ctx, types.NamespacedName{Namespace: namespace, Name: name}, tlsSecret)
	if err != nil {
		return nil, err
	}

	if tlsSecret.Type != corev1.SecretTypeTLS {
		return nil, fmt.Errorf("non tls secret found by tlsSecretRef")
	}

	certPEM := tlsSecret.Data[corev1.TLSCertKey]
	keyPEM := tlsSecret.Data[corev1.TLSPrivateKeyKey]

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}

	return &cert, nil
}

func GetListenAllAddress(port int32) string {
	portStr := strconv.FormatInt(int64(port), 10)
	return fmt.Sprintf(":%s", portStr)
}

func GetListenPort(addr string) (int32, error) {
	idx := strings.LastIndexByte(addr, ':')
	port, err := strconv.ParseInt(addr[idx+1:], 10, 32)
	if err != nil {
		return 0, err
	}
	return int32(port), nil
}
