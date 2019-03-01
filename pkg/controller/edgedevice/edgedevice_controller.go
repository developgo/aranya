package edgedevice

import (
	"context"
	"fmt"
	"net"

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

	aranyav1alpha1 "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/node"
)

const (
	controllerName = "aranya"
)

var (
	errNoFreePort = errors.NewInternalError(fmt.Errorf("could not allocate free port"))
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
	if err = c.Watch(&source.Kind{Type: &aranyav1alpha1.EdgeDevice{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	// Watch virtual nodes created by EdgeDevice object
	if err = c.Watch(&source.Kind{Type: &corev1.Node{}}, &handler.EnqueueRequestForOwner{
		IsController: true, OwnerType: &aranyav1alpha1.EdgeDevice{},
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
	reqLog.Info("reconciling edge device")

	var (
		deleted      bool
		nodeNsName   = types.NamespacedName{Name: request.Name}
		deviceNsName = types.NamespacedName{Namespace: constant.CurrentNamespace(), Name: request.Name}
	)

	// reconcile related node object
	// node name is in the form of `namespace/name`
	if request.Namespace == corev1.NamespaceAll {
		nodeObj := &corev1.Node{}
		err = r.client.Get(r.ctx, nodeNsName, nodeObj)
		if err != nil {
			if errors.IsNotFound(err) {
				reqLog.Info("node object deleted, close virtual node")
				node.DeleteRunningServer(request.Name)
			} else {
				reqLog.Error(err, "get node object failed")
				return reconcile.Result{}, err
			}
		}
	}

	// get the edge device instance
	deviceObj := &aranyav1alpha1.EdgeDevice{}
	err = r.client.Get(r.ctx, deviceNsName, deviceObj)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLog.Info("edge device deleted, clean up related objects")
			if err = r.cleanupVirtualObjects(reqLog, deviceObj); err != nil {
				reqLog.Error(err, "delete related resources failed")
				return reconcile.Result{}, err
			}

			return reconcile.Result{}, nil
		}
		reqLog.Error(err, "get edge device failed")
		return reconcile.Result{}, err
	}

	// tag with finalizer's name and do related job when necessary
	deleted, err = r.runFinalizerLogic(reqLog, deviceObj)
	if err != nil {
		return reconcile.Result{}, err
	}

	// edge device need to be deleted, no more check
	if deleted {
		return reconcile.Result{}, nil
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
	case aranyav1alpha1.DeviceConnectViaGRPC:
		svcFound := &corev1.Service{}
		err = r.client.Get(r.ctx, deviceNsName, svcFound)
		if err != nil {
			if errors.IsNotFound(err) {
				reqLog.Info("create grpc svc object")
				svcObj, grpcListener, err = r.createSvcForGrpc(deviceObj)
				if err != nil {
					reqLog.Error(err, "create grpc svc object failed")
					return reconcile.Result{}, err
				}

				defer func() {
					if err != nil {
						_ = grpcListener.Close()

						reqLog.Info("delete grpc svc object on error")
						if e := r.client.Delete(r.ctx, svcObj); e != nil {
							log.Error(e, "delete svc object failed")
						}
					}
				}()
			} else {
				reqLog.Error(err, "get svc object failed")
				return reconcile.Result{}, err
			}
		}
	}

	// check node object exists
	nodeFound := &corev1.Node{}
	err = r.client.Get(r.ctx, nodeNsName, nodeFound)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLog.Info("create node object")
			nodeObj, kubeletListener, err = r.createNodeObject(deviceObj)
			if err != nil {
				reqLog.Error(err, "create node object failed")
				return
			}

			defer func() {
				// delete related objects with best effort if error happened
				if err != nil {
					_ = kubeletListener.Close()

					reqLog.Info("delete node object on error")
					if e := r.client.Delete(r.ctx, nodeObj); e != nil {
						log.Error(e, "delete node object failed")
					}
				}
			}()

			// create and start a new virtual node instance
			reqLog.Info("create virtual node")
			virtualNode, err = node.CreateVirtualNode(r.ctx, nodeObj.DeepCopy(), kubeletListener, grpcListener, *r.config)
			if err != nil {
				reqLog.Error(err, "create virtual node failed")
				return
			}

			reqLog.Info("start virtual node")
			if err = virtualNode.Start(); err != nil {
				reqLog.Error(err, "start virtual node failed")
				return
			}
		} else {
			reqLog.Error(err, "get node object failed")
			return reconcile.Result{}, err
		}
	}

	// let arhat.dev/aranya/pkg/node.Node do update job on its own
	return reconcile.Result{}, nil
}
