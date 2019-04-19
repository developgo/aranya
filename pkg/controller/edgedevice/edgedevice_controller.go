package edgedevice

import (
	"context"
	"crypto/tls"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubeClient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	aranya "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/node"
	connectivityManager "arhat.dev/aranya/pkg/node/manager"
)

const (
	controllerName = "aranya"
)

var (
	once = &sync.Once{}
)

var log = logf.Log.WithName(controllerName)

// AddToManager creates a new EdgeDevice Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func AddToManager(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileEdgeDevice{
		client:     mgr.GetClient(),
		scheme:     mgr.GetScheme(),
		config:     mgr.GetConfig(),
		ctx:        context.Background(),
		kubeClient: kubeClient.NewForConfigOrDie(mgr.GetConfig()),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
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
	kubeClient kubeClient.Interface
	scheme     *runtime.Scheme
	config     *rest.Config
	ctx        context.Context
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
		// reconcile node only
		return reconcile.Result{}, r.doReconcileVirtualNode(reqLog, corev1.NamespaceAll, request.Name, nil)
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
		err = r.cleanupVirtualNode(reqLog, deviceObj.Namespace, deviceObj.Name, deviceObj.Name)
		if err != nil {
			reqLog.Error(err, "failed to cleanup virtual node")
			return err
		}

		return nil
	}

	//
	// edge device exists, check its related virtual node
	//
	err = r.doReconcileVirtualNode(reqLog, deviceObj.Namespace, deviceObj.Name, deviceObj)
	if err != nil {
		return err
	}

	return nil
}

func (r *ReconcileEdgeDevice) doReconcileVirtualNode(reqLog logr.Logger, namespace, nodeName string, deviceObj *aranya.EdgeDevice) (err error) {
	var (
		nodeObj      = &corev1.Node{}
		svcObj       = &corev1.Service{}
		creationOpts = &node.CreationOptions{}
		virtualNode  *node.Node

		needToCreateNodeObject  bool
		needToCreateVirtualNode bool
		ok                      bool
	)

	err = r.client.Get(r.ctx, types.NamespacedName{Namespace: corev1.NamespaceAll, Name: nodeName}, nodeObj)
	if err != nil {
		if !errors.IsNotFound(err) {
			reqLog.Error(err, "failed to get node object")
			return err
		}

		// since the node object not found (has been deleted),
		// delete the related virtual node
		reqLog.Info("node object deleted, destroy virtual node")
		node.Delete(nodeName)
		needToCreateNodeObject = true
	} else {
		nodeDeleted := !(nodeObj.DeletionTimestamp == nil || nodeObj.DeletionTimestamp.IsZero())
		if nodeDeleted {
			// node to be deleted, delete the related virtual node and return
			node.Delete(nodeName)
			return fmt.Errorf("unexpected node objected deleted")
		} else {
			// node presents and not deleted, virtual node MUST exist
			// (or we need to delete the all related objects)
			virtualNode, ok = node.Get(nodeName)
			if !ok {
				err = r.cleanupVirtualNode(reqLog, namespace, nodeName, "")
				if err != nil {
					return err
				}
				return fmt.Errorf("unexpected virtual node not present")
			} else {
				// TODO: reuse previous network listener if any
				oldOpts := virtualNode.CreationOptions()

				creationOpts.KubeletServerListener = oldOpts.KubeletServerListener
				creationOpts.GRPCServerListener = oldOpts.GRPCServerListener
				creationOpts.ConnectivityManager = oldOpts.ConnectivityManager
			}
		}
	}

	// device nil means this function was called for the node only or
	// the device has been deleted, no more action required
	if deviceObj == nil {
		return nil
	}

	// here, device presents and not deleted
	// everything related expected to exist

	if needToCreateNodeObject {
		// new node and new virtual node
		reqLog.Info("create node object")
		creationOpts.NodeObject, creationOpts.KubeletServerListener, err = r.createNodeObjectForDevice(deviceObj)
		if err != nil {
			reqLog.Error(err, "failed to create node object")
			return err
		}

		needToCreateVirtualNode = true

		defer func() {
			if err != nil {
				_ = creationOpts.KubeletServerListener.Close()

				if err := r.cleanupVirtualNode(reqLog, namespace, nodeName, deviceObj.Name); err != nil {
					return
				}
			}
		}()
	}

	// check device connectivity, check service object if grpc is used
	switch deviceObj.Spec.Connectivity.Method {
	case aranya.DeviceConnectViaGRPC:
		svcNamespacedName := types.NamespacedName{Namespace: deviceObj.Namespace, Name: deviceObj.Name}
		err = r.client.Get(r.ctx, svcNamespacedName, svcObj)
		if err != nil {
			if !errors.IsNotFound(err) {
				reqLog.Error(err, "failed to get svc object")
				return err
			}
		} else {
			// service object exists, but to be deleted,
			// return error to run another reconcile
			svcDeleted := !(svcObj.DeletionTimestamp == nil || svcObj.DeletionTimestamp.IsZero())
			if svcDeleted {
				return fmt.Errorf("unexpected service object deleted")
			}

			// service object ok, job done
			break
		}

		// service object doesn't exists,
		// whether not created with virtual node or has been deleted
		if needToCreateVirtualNode {
			// need to create service object and grpc server
			grpcConfig := deviceObj.Spec.Connectivity.Config.GRPC.ForServer
			svcObj, creationOpts.GRPCServerListener, err = r.createGRPCSvcObjectForDevice(deviceObj)
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
			if tlsRef := grpcConfig.TLSSecretRef; tlsRef != nil {
				cert, err := r.GetCertFromSecret(tlsRef.Namespace, tlsRef.Name)
				if err != nil {
					return err
				}
				grpcSrvOptions = append(grpcSrvOptions, grpc.Creds(credentials.NewServerTLSFromCert(cert)))
			}

			creationOpts.ConnectivityManager = connectivityManager.NewGRPCManager(grpc.NewServer(grpcSrvOptions...), creationOpts.GRPCServerListener)
		} else {
			// service object deleted (most likely deleted by user)
			// create service object according to existing virtual node
			if creationOpts.GRPCServerListener != nil {
				// existing virtual node work as a grpc manager
				// we just need to create a service object for it

				// NOTICE: any grpc config update will not be applied
				// TODO: should we support grpc config change?
				reqLog.Info("creating svc object for existing grpc service, no update will be applied to existing grpc service")
				port, err := GetListenPort(creationOpts.GRPCServerListener.Addr().String())
				if err != nil {
					return err
				}

				svcObj = newServiceForEdgeDevice(deviceObj, port)
				err = r.client.Create(r.ctx, svcObj)
				if err != nil {
					reqLog.Error(err, "failed to create svc object for existing grpc service")
					return err
				}
			} else {
				// existing virtual node doesn't work as grpc manager
				// changes happen in EdgeDevice's spec
				// TODO: should we support connectivity method change?
				reqLog.Info("EdgeDevice connectivity method change not supported")
			}
		}
	case aranya.DeviceConnectViaMQTT:
		if !needToCreateVirtualNode {
			// nothing to do since mqtt doesn't require any service object
			// TODO: should we support connectivity method change?
			break
		}

		mqttConfig := deviceObj.Spec.Connectivity.Config.MQTT.ForServer
		var cert *tls.Certificate
		if tlsRef := mqttConfig.TLSSecretRef; tlsRef != nil {
			cert, err = r.GetCertFromSecret(tlsRef.Namespace, tlsRef.Name)
			if err != nil {
				return err
			}
		}

		creationOpts.ConnectivityManager, err = connectivityManager.NewMQTTManager(mqttConfig, cert)
		if err != nil {
			return err
		}
	}

	// create virtual node if required
	if needToCreateVirtualNode {
		creationOpts.KubeClient = r.kubeClient

		virtualNode, err = node.CreateVirtualNode(r.ctx, creationOpts)
		if err != nil {
			reqLog.Error(err, "failed to create virtual node")
			return err
		}

		err = virtualNode.Start()
		if err != nil {
			reqLog.Error(err, "failed to start virtual node")
			return err
		}
	}

	return nil
}

func (r *ReconcileEdgeDevice) GetCertFromSecret(namespace, name string) (*tls.Certificate, error) {
	if namespace == "" {
		namespace = constant.CurrentNamespace()
	}

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
