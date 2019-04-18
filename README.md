# aranya `阿兰若`

A `Kubernetes` operator for edge devices

## Purpose

Deploy and manage edge devices with ease, integrate them into your `Kubernetes` cluster, remove the boundry between `Edge` and `Cloud`

## State

EXPRIMENT, USE AT YOUR OWN RISK

## Prerequisites

- `Kubernetes` Mode
  - `Kubernetes` Cluster with RBAC enabled
- Standalone Mode (WIP, see [ROADMAP.md](./ROADMAP.md))

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
   2. setup the connectivity between `aranya` and your edge devices
      - `gRPC`
        - `aranya` will create a `Kubernetes` service object
        - you need to create an `Ingress` object for that service if you want to access it outside the cluster
        - configure your edge device's `arhat` to connect the service
      - `MQTT`
        - `aranya` will connect to your mqtt broker
        - your edge device's `arhat`'s config should match

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
