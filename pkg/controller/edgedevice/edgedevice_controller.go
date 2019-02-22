package edgedevice

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"net"
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
	"arhat.dev/aranya/pkg/node/util"
)

const (
	controllerName = "aranya"
)

var (
	errNoFreePort = errors.NewInternalError(fmt.Errorf("could not allocat port"))
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

	// Watch virtual Node created by EdgeDevice object
	if err = c.Watch(&source.Kind{Type: &corev1.Node{}}, &handler.EnqueueRequestForOwner{
		IsController: true, OwnerType: &aranyav1alpha1.EdgeDevice{},
	}); err != nil {
		return err
	}

	// watch service created by EdgeDevice object
	if err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true, OwnerType: &aranyav1alpha1.EdgeDevice{},
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
	reqLogger := log.WithValues("request.namespace", request.Namespace, "request.name", request.Name)
	reqLogger.Info("Reconciling EdgeDevice")

	// Fetch the EdgeDevice instance
	device := &aranyav1alpha1.EdgeDevice{}
	err = r.client.Get(r.ctx, request.NamespacedName, device)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return
	}

	// tag with finalizer's name and do related job when necessary
	if device.DeletionTimestamp.IsZero() {
		if !containsString(device.Finalizers, constant.FinalizerName) {
			device.ObjectMeta.Finalizers = append(device.ObjectMeta.Finalizers, constant.FinalizerName)
			if err = r.client.Update(r.ctx, device); err != nil {
				return
			}
		}
	} else {
		if containsString(device.ObjectMeta.Finalizers, constant.FinalizerName) {
			if err = r.deleteRelatedResourceObjects(device); err != nil {
				return
			}

			device.ObjectMeta.Finalizers = removeString(device.ObjectMeta.Finalizers, constant.FinalizerName)
			if err = r.client.Update(context.Background(), device); err != nil {
				return
			}
		}

		return
	}

	// check virtual node exists
	found := &corev1.Node{}
	nodeName := util.GetVirtualNodeName(device.Name)
	err = r.client.Get(r.ctx, types.NamespacedName{Name: nodeName}, found)
	if err != nil && errors.IsNotFound(err) {
		var (
			virtualNode     *node.Node
			nodeObj         *corev1.Node
			svcObj          *corev1.Service
			kubeletListener net.Listener
			grpcListener    net.Listener
		)

		nodeObj, kubeletListener, err = r.createNodeObject(device)
		if err != nil {
			return
		}

		svcObj, grpcListener, err = r.createGrpcSvcIfUsed(device)
		if err != nil {
			return
		}

		defer func() {
			if err != nil {
				// delete related objects with best effort
				if e := r.client.Delete(r.ctx, svcObj); e != nil {
					log.Error(e, "delete svc object failed")
				}

				if e := r.client.Delete(r.ctx, nodeObj); e != nil {
					log.Error(e, "delete node object failed")
				}
			}
		}()

		// create and start a new virtual node instance
		virtualNode, err = node.CreateVirtualNode(r.ctx, *nodeObj, kubeletListener, grpcListener, *r.config)
		if err != nil {
			reqLogger.Error(err, "create virtual node failed")
			return
		}

		if err = virtualNode.Start(); err != nil {
			reqLogger.Error(err, "start virtual node failed")
			return
		}

		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// let arhat.dev/aranya/pkg/node.Node do update job on its own
	return reconcile.Result{}, nil
}
