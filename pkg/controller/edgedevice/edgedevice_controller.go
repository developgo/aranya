package edgedevice

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
}

// Reconcile reads that state of the cluster for a EdgeDevice object and makes changes based on the state read
// and what is in the EdgeDevice.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileEdgeDevice) Reconcile(request reconcile.Request) (result reconcile.Result, err error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling EdgeDevice")

	// Fetch the EdgeDevice instance
	device := &aranyav1alpha1.EdgeDevice{}
	err = r.client.Get(context.TODO(), request.NamespacedName, device)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// tag with finalizer's name and do related job when necessary
	if device.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object.
		if !containsString(device.Finalizers, constant.FinalizerName) {
			device.ObjectMeta.Finalizers = append(device.ObjectMeta.Finalizers, constant.FinalizerName)
			if err := r.client.Update(context.TODO(), device); err != nil {
				return reconcile.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if containsString(device.ObjectMeta.Finalizers, constant.FinalizerName) {
			// our finalizer is present, so lets handle our external dependency
			if err := r.deleteRelatedVirtualNode(device); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return reconcile.Result{}, err
			}

			// remove our finalizer from the list and update it.
			device.ObjectMeta.Finalizers = removeString(device.ObjectMeta.Finalizers, constant.FinalizerName)
			if err := r.client.Update(context.Background(), device); err != nil {
				return reconcile.Result{}, err
			}
		}

		return reconcile.Result{}, nil
	}

	// get node ip address
	hostIP, err := r.getHostIP(reqLogger)
	if err != nil {
		return reconcile.Result{}, err
	}

	// get free port on this node
	kubeletListenPort := getFreePort()
	if kubeletListenPort < 1 {
		return reconcile.Result{}, errNoFreePort
	}

	// TODO: add configuration option to disable allocation for grpc port
	grpcListenPort := getFreePort()
	if grpcListenPort < 1 {
		return reconcile.Result{}, errNoFreePort
	}

	newSvc := newServiceForEdgeDevice(device, grpcListenPort)
	err = controllerutil.SetControllerReference(device, newSvc, r.scheme)
	if err != nil {
		log.Error(err, "set svc controller reference failed")
		return reconcile.Result{}, err
	}

	newVirtualNode := newNodeForEdgeDevice(device, hostIP, kubeletListenPort)
	err = controllerutil.SetControllerReference(device, newVirtualNode, r.scheme)
	if err != nil {
		log.Error(err, "set node controller reference failed")
		return reconcile.Result{}, err
	}

	// check virtual node exists
	found := &corev1.Node{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: newVirtualNode.Name, Namespace: newVirtualNode.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Node", "node.name", newVirtualNode.Name)

		// create and start a new kubelet instance
		kubeletListenAddress := fmt.Sprintf("%s:%d", hostIP, kubeletListenPort)
		grpcListenAddress := fmt.Sprintf(":%d", grpcListenPort)
		srv, err := node.CreateVirtualNode(context.TODO(),
			newVirtualNode.Name, kubeletListenAddress, grpcListenAddress, r.config, device, newVirtualNode, newSvc)
		if err != nil {
			reqLogger.Error(err, "CreateVirtualNode failed")
			return reconcile.Result{}, err
		}

		if err = srv.StartListenAndServe(); err != nil {
			reqLogger.Error(err, "StartListenAndServe node failed")
			return reconcile.Result{}, err
		}

		// create the virtual node object
		err = r.client.Create(context.TODO(), newVirtualNode)
		if err != nil {
			reqLogger.Error(err, "Could not create Node")
			// close server
			srv.ForceClose()
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// let arhat.dev/aranya/pkg/node.Node do update job on its own
	return reconcile.Result{}, nil
}

func (r *ReconcileEdgeDevice) getHostIP(reqLogger logr.Logger) (addr string, err error) {
	reqLogger.Info("Get Node InternalIP")
	currentPod := &corev1.Pod{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: constant.CurrentNamespace(), Name: constant.CurrentPodName()}, currentPod)
	if err != nil {
		reqLogger.Error(err, "Could not get current Pod", "Pod.Name", constant.CurrentPodName(), "Pod.Namespace", constant.CurrentNamespace())
		return "", err
	}

	return currentPod.Status.HostIP, nil
}

func (r *ReconcileEdgeDevice) deleteRelatedVirtualNode(device *aranyav1alpha1.EdgeDevice) error {
	nodeName := util.GetVirtualNodeName(device.Name)

	virtualNode := &corev1.Node{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: nodeName}, virtualNode)
	if err != nil {
		return err
	}

	err = r.client.Delete(context.TODO(), virtualNode)
	if err != nil {
		return err
	}

	node.DeleteRunningServer(nodeName)

	return nil
}
