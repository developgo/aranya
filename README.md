# aranya `阿兰若`

A `Kubernetes` operator for edge devices

## Purpose

Deploy and manage edge devices with ease, integrate them into your `Kubernetes` cluster, remove the boundry between `Edge` and `Cloud`

## State

EXPERIMENTAL, USE AT YOUR OWN RISK

## Prerequisites

- `Kubernetes` Cluster with RBAC enabled

## Features

- `Kubernetes` Mode
  - Full featured `Kubernetes` workload executor (except Network)
    - Support `Pod` creation with `Env`, `Volume`
      - Support source from plain text, `Secret` and `ConfigMap`
    - Support `kubectl`'s `log`, `exec`, `attach`, `portforward`

## Workflow

1. Deploy `aranya` to your `Kubernetes` cluster with following commands

   ```bash
   # set the namespace for edge devices, aranya will be deployed to this namespace
   $ export NS=edge

   # create the namespace
   $ kubectl create namespace ${NS}

   # create custom resource definitions used by aranya
   $ kubectl apply -f https://raw.githubusercontent.com/arhat-dev/aranya/master/cicd/k8s/crds/aranya_v1alpha1_edgedevice_crd.yaml

   # create cluster role for aranya
   $ kubectl apply -f https://raw.githubusercontent.com/arhat-dev/aranya/master/cicd/k8s/aranya-cluster-role.yaml

   # create service account for aranya
   $ kubectl -n ${NS} create serviceaccount aranya

   # config RBAC for aranya
   $ kubectl create clusterrolebinding --clusterrole=aranya --serviceaccount=${NS}:aranya

   # deploy aranya to your cluster
   $ kubectl -n ${NS} apply -f https://raw.githubusercontent.com/arhat-dev/aranya/master/cicd/k8s/aranya-deploy.yaml
   ```

2. Create a `EdgeDevice` resource object for each one of your edge devices (see [cicd/k8s/sample/sample-edge-devices.yaml](./cicd/k8s/sample/sample-edge-devices.yaml) for example)
   1. `aranya` will create a `Kubernetes` node object according to the edge device spec in cluster
   2. setup the connectivity between `aranya` and your edge devices, depending on the connectivity method set in the spec (`spec.connectivity.method`):
      - `grpc`
        - A gRPC server will be created and served by `aranya` according to the `spec.connectivity.grpcConfig`, `aranya` also maintains an according service object for that server.
        - If you want to access the newly created gRPC service for your edge device outside the cluster, you need to setup `Kubernetes Ingress` using applications like [`ingress-nginx`](https://github.com/kubernetes/ingress-nginx), [`traefik`](https://github.com/containous/traefik) etc. at first. Then you need to create an `Ingress` object (see [exmaple](./cicd/k8s/sample/sample-ingress-traefik.yaml)) for the gRPC service.
        - Configure your edge device's `arhat` to connect the gRPC accoding to your `Ingress`'s host
      - `mqtt`
        - `aranya` will try to talk to your mqtt broker accoding to the `spec.connectivity.mqttConfig`.
        - You need to configure your edge device's `arhat` to talk to the same mqtt broker or one broker in the same mqtt broker cluster depending on your own usecase, the config option `messageNamespace` must match to get `arhat` able to communicate with `aranya`.

3. Create `Kubernetes` workloads with special taints and label selector (see [cicd/k8s/sample/sample-workload.yaml](./cicd/k8s/sample/sample-workload.yaml) for example)
   - Taints

      | Taint Key             | Value                                     |
      | --------------------- | ----------------------------------------- |
      | `arhat.dev/namespace` | The namespace the edge device deployed to |

   - (Node) Labels

      | Label Name       | Value                |
      | ---------------- | -------------------- |
      | `arhat.dev/role` | `EdgeDevice`         |
      | `arhat.dev/name` | The edge device name |

## Q&A

- Why not `k3s`?
  - `k3s` is really awesome for some kind of edge devices, but still needs a lot of work to be done to serve all kinds of edge devices right, one significant problem is the splited networks with NAT, and we don't think problems like that should be resolved in `k3s` project wich would totally change the way `k3s` works.
- Why not `virtual-kubelet`?
  - `virtual-kubelet` is great for cloud providers such as `Azure`, `GCP`, `AWS` etc. and also suitable for `openstack` cluster users, however, most edge device users don't have or don't want to setup such kind of cloud infrastructure.
  - `virtual-kubelt` deployed as a pod, if applied for edge device, large scale edge device cluster will calim a lot of pod resource, which requires a lot of node to serve.
- Why `arhat` and `aranya` (why not `kubelet`)?
  - `kubelet` is heavily dependent on http, maybe it's not a good idea for edge devices with poor network to communicate with each other via http.
  - `aranya` is the watcher part in `kubelet`, lots of `kubelet`'s work such as cluster resource fetch and update is done by `aranya`, `aranya` resolves everything before command was delivered to `arhat`.
  - `arhat` is the worker part in `kubelet`, it's an event driven agent and only tend to command execution.
  - Due to the design decisions above, we only need to transfer necessary messages between `aranya` and `arhat` such as pod creation command, container status update, node status update etc. Keeping the edge device's data usage for management as low as possible.

## Performance

Every `EdgeDevice` object needs to setup a `kubelet` server serving `kubectl` commands such as `logs`, `exec`, `attach` and `port-forward`, we need to request node certificate for `EdgeDevice`'s virtual node in any `RBAC` enabled cluster, which would take a lot of time for lage scale deployment. The performance test was taken on my own `Kubernetes` cluster described in [my `homelab`](https://github.com/jeffreystoke/homelab) after all the required node certificates has been provisioned.

- Test Workload
  - 1000 EdgeDevice using `gRPC` (generated with `./scripts/gen-deploy-script.sh 1000`)
    - each requires a `gRPC` and `kubelet` server
    - each requires a `Node` and `Service` object
- Resuts

  ```txt
  Deployment Speed:   ~ 5 devices/s
  Delete Speed:       ~ 6 devices/s
  ```

However, after 1000 devices has been deployed and 1000 according node objects created, my cluster shuts me out due to the `kube-apiserver` unable to handle, hope to be tested on more performant platforms.

## Roadmap

see [ROADMAP.md](./ROADMAP.md)

## Thanks to

This project was inspired by [`virtual-kubelet`](https://github.com/virtual-kubelet/virtual-kubelet)'s idea, which introduced an cloud agent to run containers in network edge.

## Authors

- [Jeffrey Stoke](https://github.com/jeffreystoke) (project owner)
  - I'm looking for jobs (associate to junior level) in Deutschland
