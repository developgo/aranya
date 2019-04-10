package edgedevice

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
)

const (
	controllerName = "aranya"
)

var (
	errNoFreePort = errors.NewInternalError(fmt.Errorf("could not allocate free port"))
	once          = &sync.Once{}
)

var log = logf.Log.WithName(controllerName)

// Add creates a new EdgeDevice Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileEdgeDevice{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		config: mgr.GetConfig(),
		ctx:    context.TODO(),
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

	// Watch virtual nodes created by EdgeDevice object
	if err = c.Watch(&source.Kind{Type: &corev1.Node{}}, &handler.EnqueueRequestForOwner{
		IsController: true, OwnerType: &aranya.EdgeDevice{},
	}); err != nil {
		return err
	}

	// watch services created by EdgeDevice object
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
	client client.Client
	scheme *runtime.Scheme
	config *rest.Config
	ctx    context.Context
}

// Reconcile reads that state of the cluster for a EdgeDevice object and makes changes based on the state read
// and what is in the EdgeDevice.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileEdgeDevice) Reconcile(request reconcile.Request) (result reconcile.Result, err error) {
	reqLog := log.WithValues("ns", request.Namespace, "name", request.Name)
	once.Do(func() {
		reqLog.Info("initialize edge devices")
		// get all edge devices in this namespace (only once)
		deviceList := &aranya.EdgeDeviceList{}
		err = r.client.List(r.ctx, &client.ListOptions{Namespace: constant.CurrentNamespace()}, deviceList)
		if err != nil {
			reqLog.Error(err, "failed to list edge devices")
			return
		}

		for _, device := range deviceList.Items {
			if err := r.doReconcileEdgeDevice(reqLog, &device); err != nil {
				reqLog.Error(err, "reconcile edge device failed", "device.name", device.Name)
			}
		}
	})
	if err != nil {
		return
	}

	var (
		nodeNsName   = types.NamespacedName{Namespace: corev1.NamespaceAll, Name: request.Name}
		deviceNsName = types.NamespacedName{Namespace: constant.CurrentNamespace(), Name: request.Name}
	)

	// reconcile related node object
	if request.Namespace == corev1.NamespaceAll {
		nodeObj := &corev1.Node{}
		err = r.client.Get(r.ctx, nodeNsName, nodeObj)
		if err != nil {
			if errors.IsNotFound(err) {
				reqLog.Info("node object deleted, destroy virtual node")
				// since the node object has been deleted, delete the virtual node only
				node.Delete(request.Name)
			} else {
				reqLog.Error(err, "failed to get node object")
				return reconcile.Result{}, err
			}
		}

		return reconcile.Result{}, nil
	}

	// get the edge device instance
	deviceObj := &aranya.EdgeDevice{}
	err = r.client.Get(r.ctx, deviceNsName, deviceObj)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLog.Info("edge device deleted, clean up related objects")
			if err = r.cleanupEdgeDeviceAndVirtualNode(reqLog, deviceObj); err != nil {
				reqLog.Error(err, "failed to cleanup related resources")
				return reconcile.Result{}, err
			}

			return reconcile.Result{}, nil
		}

		reqLog.Error(err, "failed to get edge device")
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, r.doReconcileEdgeDevice(reqLog, deviceObj)
}

func (r *ReconcileEdgeDevice) doReconcileEdgeDevice(reqLog logr.Logger, deviceObj *aranya.EdgeDevice) (err error) {
	deviceDeleted := !(deviceObj.DeletionTimestamp == nil || deviceObj.DeletionTimestamp.IsZero())

	// edge device need to be deleted, no more check
	if deviceDeleted {
		if err = r.cleanupEdgeDeviceAndVirtualNode(reqLog, deviceObj); err != nil {
			reqLog.Error(err, "failed to related resources")
			return err
		}

		return nil
	}

	//
	// edge device exists, check its dependent objects
	//

	var (
		virtualNode     *node.Node
		nodeObj         *corev1.Node
		svcObj          *corev1.Service
		grpcListener    net.Listener
		kubeletListener net.Listener
	)

	// check service object if grpc is used
	switch deviceObj.Spec.Connectivity.Method {
	case aranya.DeviceConnectViaGRPC:
		svcFound := &corev1.Service{}
		err := r.client.Get(r.ctx, types.NamespacedName{Namespace: deviceObj.Namespace, Name: deviceObj.Name}, svcFound)
		if err != nil {
			if !errors.IsNotFound(err) {
				reqLog.Error(err, "failed to get svc object")
				return err
			}

			// svc object not found, create one

			reqLog.Info("create grpc svc object")
			svcObj, grpcListener, err = r.createSvcForGrpc(deviceObj)
			if err != nil {
				reqLog.Error(err, "failed to create grpc svc object")
				return err
			}

			defer func() {
				if err != nil {
					_ = grpcListener.Close()

					reqLog.Info("delete grpc svc object on error")
					if e := r.client.Delete(r.ctx, svcObj); e != nil {
						log.Error(e, "failed to delete svc object")
					}
				}
			}()
		}
	}

	// check the node object exists
	nodeFound := &corev1.Node{}
	err = r.client.Get(r.ctx, types.NamespacedName{Namespace: corev1.NamespaceAll, Name: deviceObj.Name}, nodeFound)
	if err != nil {
		if !errors.IsNotFound(err) {
			reqLog.Error(err, "failed to get node object")
			return err
		}

		// node not found, create one

		reqLog.Info("create node object since not found")
		nodeObj, kubeletListener, err = r.createNodeObject(deviceObj)
		if err != nil {
			reqLog.Error(err, "failed to create node object")
			return err
		}

		defer func() {
			// delete related objects with best effort if error happened
			if err != nil {
				_ = kubeletListener.Close()

				reqLog.Info("delete node object on error")
				if e := r.client.Delete(r.ctx, nodeObj); e != nil {
					reqLog.Error(e, "failed to delete node object")
				}
			}
		}()

		// create and start a new virtual node instance
		reqLog.Info("create virtual node for node object")
		virtualNode, err = node.CreateVirtualNode(r.ctx, nodeObj.DeepCopy(), kubeletListener, grpcListener, *r.config)
		if err != nil {
			reqLog.Error(err, "failed to create virtual node")
			return err
		}

		reqLog.Info("start virtual node")
		if err = virtualNode.Start(); err != nil {
			reqLog.Error(err, "failed to start virtual node")
			return err
		}
	}

	return nil
}
