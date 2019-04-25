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

package virtualnode

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeClient "k8s.io/client-go/kubernetes"
	kubeNodeClient "k8s.io/client-go/kubernetes/typed/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"arhat.dev/aranya/pkg/connectivity"
	"arhat.dev/aranya/pkg/connectivity/server"
	"arhat.dev/aranya/pkg/virtualnode/pod"
	"arhat.dev/aranya/pkg/virtualnode/util"
)

var log = logf.Log.WithName("node")

type CreationOptions struct {
	// required fields
	NodeObject            *corev1.Node
	KubeClient            kubeClient.Interface
	KubeletServerListener net.Listener

	Manager server.Manager
	Config  *Config

	// optional
	GRPCServerListener net.Listener
}

func CreateVirtualNode(parentCtx context.Context, opt *CreationOptions) (*VirtualNode, error) {
	ctx, exit := context.WithCancel(parentCtx)
	nodeLogger := log.WithValues("name", opt.NodeObject.Name)

	m := &mux.Router{NotFoundHandler: util.NotFoundHandler(nodeLogger)}
	// register http routes

	m.Use(util.LogMiddleware(nodeLogger), util.PanicRecoverMiddleware(nodeLogger))
	m.StrictSlash(true)
	//
	// routes for pod
	//
	// logs (kubectl cluster-info dump)
	// m.Handle("/logs/{logpath:*}", http.StripPrefix("/logs/", http.FileServer(http.Dir("/var/log/")))).Methods(http.MethodGet)

	// handle pod related
	podManager := pod.NewManager(ctx, opt.NodeObject.Name, opt.KubeClient, opt.Manager, &opt.Config.Stream, &opt.Config.Pod)
	// containerLogs (kubectl logs)
	m.HandleFunc("/containerLogs/{namespace}/{name}/{container}", podManager.HandlePodContainerLog).Methods(http.MethodGet)
	// exec (kubectl exec)
	m.HandleFunc("/exec/{namespace}/{name}/{container}", podManager.HandlePodExec).Methods(http.MethodPost, http.MethodGet)
	m.HandleFunc("/exec/{namespace}/{name}/{uid}/{container}", podManager.HandlePodExec).Methods(http.MethodPost, http.MethodGet)
	// attach (kubectl attach)
	m.HandleFunc("/attach/{namespace}/{name}/{container}", podManager.HandlePodAttach).Methods(http.MethodPost, http.MethodGet)
	m.HandleFunc("/attach/{namespace}/{name}/{uid}/{container}", podManager.HandlePodAttach).Methods(http.MethodPost, http.MethodGet)
	// portForward (kubectl proxy)
	m.HandleFunc("/portForward/{namespace}/{name}", podManager.HandlePodPortForward).Methods(http.MethodPost, http.MethodGet)
	m.HandleFunc("/portForward/{namespace}/{name}/{uid}", podManager.HandlePodPortForward).Methods(http.MethodPost, http.MethodGet)

	// TODO: handle metrics
	// m.HandleFunc("/metrics", ).Methods(http.MethodGet)
	// m.HandleFunc("/metrics/cadvisor", ).Methods(http.MethodGet)

	// TODO: handle stats
	// m.HandleFunc("/stats/summary", ).Methods(http.MethodGet)

	srv := &VirtualNode{
		opt: opt,

		log:  nodeLogger,
		ctx:  ctx,
		exit: exit,
		name: opt.NodeObject.Name,

		kubeNodeClient: opt.KubeClient.CoreV1().Nodes(),

		kubeletSrv:      &http.Server{Handler: m},
		podManager:      podManager,
		nodeStatusCache: newNodeCache(opt.NodeObject.Status),
	}

	return srv, nil
}

type VirtualNode struct {
	once sync.Once
	opt  *CreationOptions

	log  logr.Logger
	ctx  context.Context
	exit context.CancelFunc
	name string

	kubeNodeClient kubeNodeClient.NodeInterface

	kubeletSrv      *http.Server
	podManager      *pod.Manager
	nodeStatusCache *NodeCache

	// status
	status uint32
	mu     sync.RWMutex
}

func (vn *VirtualNode) Start() (err error) {
	err = fmt.Errorf("server has started, do not start again")
	vn.once.Do(func() {
		if err = Add(vn); err != nil {
			vn.log.Error(err, "failed to add virtual node to collections")
			return
		}

		// start a kubelet http server
		go func() {
			vn.log.Info("starting kubelet http server")
			defer func() {
				vn.log.Info("kubelet http server exited")

				// once kubelet server exited, delete this virtual node
				Delete(vn.name)
			}()

			if err := vn.kubeletSrv.Serve(vn.opt.KubeletServerListener); err != nil && err != http.ErrServerClosed {
				vn.log.Error(err, "failed to start kubelet http server")
				return
			}
		}()

		go func() {
			vn.log.Info("starting connectivity manager")
			defer func() {
				vn.log.Info("connectivity manager exited")

				// once connectivity manager exited, delete this virtual node
				Delete(vn.name)
			}()

			if err := vn.opt.Manager.Start(); err != nil {
				vn.log.Error(err, "failed to start connectivity manager")
				return
			}
		}()

		go func() {
			vn.log.Info("starting pod manager")
			defer func() {
				vn.log.Info("pod manager exited")

				// once connectivity manager exited, delete this virtual node
				Delete(vn.name)
			}()

			if err := vn.podManager.Start(); err != nil {
				vn.log.Error(err, "failed to start pod manager")
				return
			}
		}()

		// initialize remote device
		go func() {
			vn.log.Info("starting to handle device connect")
			defer func() {
				vn.log.Info("stopped waiting for device connect")

				// once connectivity manager exited, delete this virtual node and the according node object
				Delete(vn.name)

				vn.log.Info("trying to delete node object by virtual node")
				err := vn.kubeNodeClient.Delete(vn.name, metav1.NewDeleteOptions(0))
				if err != nil && !errors.IsNotFound(err) {
					vn.log.Error(err, "failed to delete node object by virtual node")
				}
			}()

			var chanClosed uint32
			for !vn.closing() {
				select {
				case <-vn.opt.Manager.Connected():
					// we are good to go
					vn.log.Info("device connected, starting to handle")
				case <-vn.ctx.Done():
					return
				}

				go func() {
					vn.log.Info("starting to handle global messages")
					defer vn.log.Info("stopped handling global messages")

					for {
						select {
						case <-vn.opt.Manager.Disconnected():
							return
						case <-vn.ctx.Done():
							return
						case msg, more := <-vn.opt.Manager.GlobalMessages():
							if !more {
								return
							}
							vn.log.Info("recv global message")
							vn.handleGlobalMsg(msg)
						}
					}
				}()

				// mark channel not closed
				atomic.StoreUint32(&chanClosed, 0)
				// combine multiple signals into one
				stopSig := make(chan struct{})
				go func() {
					select {
					case <-stopSig:
						// can be closed else where
						return
					case <-vn.opt.Manager.Disconnected():
						// device disconnected (if rejected will also trigger disconnect)
						if atomic.CompareAndSwapUint32(&chanClosed, 0, 1) {
							close(stopSig)
						}
					case <-vn.ctx.Done():
						if atomic.CompareAndSwapUint32(&chanClosed, 0, 1) {
							close(stopSig)
						}
					}
				}()

				vn.log.Info("trying to sync device info for the first time")
				if err := vn.syncDeviceNodeStatus(); err != nil {
					vn.log.Error(err, "failed to sync device node info, reject")
					vn.opt.Manager.Reject(connectivity.RejectedByNodeStatusSyncError, "failed to pass initial node status sync")
					goto waitForDeviceDisconnect
				}

				vn.log.Info("trying to sync device pods for the first time")
				if err := vn.podManager.SyncDevicePods(); err != nil {
					vn.log.Error(err, "failed to sync device pods")
					vn.opt.Manager.Reject(connectivity.RejectedByPodStatusSyncError, "failed to pass initial pods sync")
					goto waitForDeviceDisconnect
				}

				vn.log.Info("starting to handle mirror node status update")
				go wait.Until(vn.syncMirrorNodeStatus, vn.opt.Config.Node.Timers.StatusSyncInterval, stopSig)

				// start force node status check if configured
				if vn.opt.Config.Connectivity.Timers.ForceNodeStatusSyncInterval > 0 {
					go wait.Until(func() {
						vn.log.Info("trying to force sync device node info")
						if err := vn.syncDeviceNodeStatus(); err != nil {
							// failed to sync device node status, device down,
							// wait for another connection
							vn.log.Error(err, "failed to force sync device node info")
							vn.opt.Manager.Reject(connectivity.RejectedByNodeStatusSyncError, "failed to pass force node status sync")
							if atomic.CompareAndSwapUint32(&chanClosed, 0, 1) {
								close(stopSig)
							}
							return
						}
					}, vn.opt.Config.Connectivity.Timers.ForceNodeStatusSyncInterval, stopSig)
				}

				// start force pods check if configured
				if vn.opt.Config.Connectivity.Timers.ForcePodStatusSyncInterval > 0 {
					go wait.Until(func() {
						vn.log.Info("trying to force sync device pods")
						if err := vn.podManager.SyncDevicePods(); err != nil {
							// failed to sync device node status, device down
							vn.log.Error(err, "failed to force sync device pods")
							vn.opt.Manager.Reject(connectivity.RejectedByNodeStatusSyncError, "failed to pass force pods sync")
							if atomic.CompareAndSwapUint32(&chanClosed, 0, 1) {
								close(stopSig)
							}
							return
						}
					}, vn.opt.Config.Connectivity.Timers.ForcePodStatusSyncInterval, stopSig)
				}
			waitForDeviceDisconnect:
				<-stopSig
			}
		}()
	})

	return err
}

// ForceClose close this node immediately
func (vn *VirtualNode) ForceClose() {
	vn.mu.Lock()
	defer vn.mu.Unlock()

	vn.log.Info("force close virtual node")

	_ = vn.kubeletSrv.Close()
	vn.opt.Manager.Stop()
	vn.podManager.Stop()
	vn.exit()
}

func (vn *VirtualNode) CreationOptions() CreationOptions {
	vn.mu.RLock()
	defer vn.mu.RUnlock()

	return *vn.opt
}

func (vn *VirtualNode) closing() bool {
	select {
	case <-vn.ctx.Done():
		return true
	default:
		return false
	}
}
