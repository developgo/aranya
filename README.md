# aranya `阿兰若`

[![Build Status](https://travis-ci.com/arhat-dev/aranya.svg)](https://travis-ci.com/arhat-dev/aranya) [![GoDoc](https://godoc.org/github.com/arhat-dev/aranya?status.svg)](https://godoc.org/arhat.dev/aranya) [![GoReportCard](https://goreportcard.com/badge/github.com/arhat-dev/aranya)](https://goreportcard.com/report/arhat.dev/aranya) [![codecov](https://codecov.io/gh/arhat-dev/aranya/branch/master/graph/badge.svg)](https://codecov.io/gh/arhat-dev/aranya)

A `Kubernetes` operator for edge devices

(This project also includes `arhat`'s source, which is the agent for edge device to communicate with `aranya`)

## Purpose

- Deploy and manage edge devices with ease.
- Remove the boundry between `Edge` and `Cloud`.
- Integrate every device with container runtime into your `Kubernetes` cluster.

## Non-Purpose

Simplify `Kubernetes`

## State

EXPERIMENTAL, USE AT YOUR OWN RISK

## Features

- Pod modeled container management in edge device
  - Support `Pod` creation with `Env`, `Volume`
    - Sources: plain text, `Secret`, `ConfigMap`
- Remote management with `kubectl`
  - `log`
  - `exec`
    - `arhat` treats commands with prefix `#` as host command, useful for remote device management
      - e.g. `kubectl exec -it example-pod \#bash` will get you into a host `bash`
  - `attach`
  - `port-forward`

## Restrictions

- `Kubernetes` cluster network not working for edge devices, see [Roadmap #Networking](./ROADMAP.md#Networking)

## Build

see [docs/Build.md](./docs/Build.md)

## Deployment Prerequisites

- `Kubernetes` cluster with `RBAC` enabled
  - Minimum cluster requirements: 1 master (must have) with 1 node (to deploy `aranya`)

## Deployment Workflow

1. Deploy `aranya` to your `Kubernetes` cluster for test with following commands (see [docs/Maintenance.md](./docs/Maintenance.md) for more deployment tips)

   ```bash
   # set the namespace for edge devices, aranya will be deployed to this namespace
   $ export NS=edge

   # create the namespace
   $ kubectl create namespace ${NS}

   # create custom resource definitions used by aranya
   $ kubectl apply -f https://raw.githubusercontent.com/arhat-dev/aranya/master/cicd/k8s/crds/aranya_v1alpha1_edgedevice_crd.yaml

   # create cluster role for aranya
   $ kubectl apply -f https://raw.githubusercontent.com/arhat-dev/aranya/master/cicd/k8s/aranya-roles.yaml

   # create service account for aranya
   $ kubectl -n ${NS} create serviceaccount aranya

   # config RBAC for aranya
   $ kubectl create clusterrolebinding aranya --clusterrole=aranya --serviceaccount=${NS}:aranya

   # deploy aranya to your cluster
   $ kubectl -n ${NS} apply -f https://raw.githubusercontent.com/arhat-dev/aranya/master/cicd/k8s/aranya-deploy.yaml
   ```

2. Create `EdgeDevice` resource objects for each one of your edge devices (see [sample-edge-devices.yaml](./cicd/k8s/sample/sample-edge-devices.yaml) for example)
   1. `aranya` will create a node object with the same name for every `EdgeDevice` in your cluster
   2. Configure the connectivity between `aranya` and your edge devices, depending on the connectivity method set in the spec (`spec.connectivity.method`):
      - `grpc`
        - A gRPC server will be created and served by `aranya` according to the `spec.connectivity.grpcConfig`, `aranya` also maintains an according service object for that server.
        - If you want to access the newly created gRPC service for your edge device outside the cluster, you need to setup `Kubernetes Ingress` using applications like [`ingress-nginx`](https://github.com/kubernetes/ingress-nginx), [`traefik`](https://github.com/containous/traefik) etc. at first. Then you need to create an `Ingress` object (see [sample-ingress-traefik.yaml](./cicd/k8s/sample/sample-ingress-traefik.yaml) for example) for the gRPC service.
        - Configure your edge device's `arhat` to connect the gRPC server accoding to your `Ingress`'s host
      - `mqtt` (WIP, see [Roadmap #Connectivity](./ROADMAP.md#Connectivity))
        - `aranya` will try to talk to your mqtt broker accoding to the `spec.connectivity.mqttConfig`.
        - You need to configure your edge device's `arhat` to talk to the same mqtt broker or one broker in the same mqtt broker cluster depending on your own usecase, the config option `messageNamespace` must match to get `arhat` able to communicate with `aranya`.
   3. Deploy `arhat` with configuration to your edge devices, start and wait to get connected to `aranya`

3. Create workloads with tolerations (taints for edge devices) and use label selectors or node affinity to assign to specific edge devices (see [sample-workload.yaml](./cicd/k8s/sample/sample-workload.yaml) for example)
   - Common Node Taints

      | Taint Key             | Value                                             |
      | --------------------- | ------------------------------------------------- |
      | `arhat.dev/namespace` | Name of the namespace the edge device deployed to |

   - Common Node Labels

      | Label Name       | Value                |
      | ---------------- | -------------------- |
      | `arhat.dev/role` | `EdgeDevice`         |
      | `arhat.dev/name` | The edge device name |

## Performance

Every `EdgeDevice` object needs to setup a `kubelet` server to serve `kubectl` commands which could execute into certain pods, thus we need to provision node certifications for each one of `EdgeDevice`s' virtual node in cluster, which would take a lot of time for lage scale deployment. The performance test was taken on my own `Kubernetes` cluster described in [my homelab](https://github.com/jeffreystoke/homelab) after all the required node certifications has been provisioned.

- Test Workload
  - 1000 EdgeDevice using `gRPC` (generated with `./scripts/gen-deploy-script.sh 1000`)
    - each requires a `gRPC` and `kubelet` server
    - each requires a `Node` and `Service` object
- Resuts

  ```txt
  ---
  Deployment Speed:   ~ 5 devices/s
  Memory Usage:       ~ 280 MB
  CPU Usage:          ~ 3 GHz

  ---
  Delete Speed:       ~ 6 devices/s
  ```

However, after 1000 devices and node objects deployed and serving, my cluster shuts me out due to the `kube-apiserver` unable to handle more requests, but it's farely good result for my 4 core virtual machine serving both `etcd` and `kube-apiserver`.

## Roadmap

see [ROADMAP.md](./ROADMAP.md)

## Q&A

- Why not `k3s`?
  - `k3s` is really awesome for some kind of edge devices, but still requires lots of work to be done to serve all kinds of edge devices right. One of the most significant problems is the splited networks with NAT or Firewall (such as homelab), and we don't think problems like that should be resolved in `k3s` project which would totally change the way `k3s` works.
- Why not using `virtual-kubelet`?
  - `virtual-kubelet` is designed for cloud providers such as `Azure`, `GCP`, `AWS` to run containers at network edge. However, most edge device users aren't able to or don't want to setup such kind of network infrastructure.
  - A `virtual-kubelet` is deployed as a pod on behalf of a contaienr runtime, if this model is applied for edge devices, then large scale edge device cluster would claim a lot of pod resource, which requires a lot of node to serve, it's inefficient.
- Why `arhat` and `aranya` (why not `kubelet`)?
  - `kubelet` is heavily dependent on http, maybe it's not a good idea for edge devices with poor network to communicate with each other via http.
  - `aranya` is the watcher part in `kubelet`, lots of `kubelet`'s work such as cluster resource fetch and update is done by `aranya`, `aranya` resolves everything in cluster for `arhat` before any command was delivered to `arhat`.
  - `arhat` is the worker part in `kubelet`, it's an event driven agent and only tend to command execution.
  - Due to the design decisions above, we only need to transfer necessary messages between `aranya` and `arhat` such as pod creation command, container status update, node status update etc. Keeping the edge device's data usage for management as low as possible.

## Thanks to

- [`Kubernetes`](https://github.com/kubernetes/kubernetes)
  - Really eased my life with my homelab.
- [`virtual-kubelet`](https://github.com/virtual-kubelet/virtual-kubelet)
  - This project was inspired by its idea, which introduced an cloud agent to run containers in network edge.
- `Buddhism`
  - Which is the origin of the name `aranya` and `arhat`.

## Authors

- [Jeffrey Stoke](https://github.com/jeffreystoke)
  - I'm seeking for career opportunities (associate to junior level) in Deutschland

## License

[![GitHub license](https://img.shields.io/github/license/arhat-dev/aranya.svg)](https://github.com/arhat-dev/aranya/blob/master/LICENSE.txt)

```text
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
```
