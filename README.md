# Aranya 阿兰若

A `Kubernetes` operator for edge devices

## Purpose

Deploy your edge devices with ease and integrate them into your own `Kubernetes` cluster as a virtual cluster (namespace)

## Workflow

1. Deploy `aranya` to your `Kubernetes` cluster with following commands

   ```bash
   # set the namespace for edge devices, aranya will be deployed to this namespace
   export NS=edge
   # create the namespace
   $ kubectl create namespace ${NS}
   # create custom resource definitions used by aranya
   $ kubectl apply -f https://github.com/arhat-dev/aranya/blob/master/cicd/k8s/crds/aranya_v1alpha1_edgedevice_crd.yaml
   # create cluster role for aranya
   $ kubectl apply -f https://github.com/arhat-dev/aranya/blob/master/cicd/k8s/aranya-cluster-role.yaml
   # create service account for aranya
   $ kubectl -n ${NS} create serviceaccount aranya
   # config RBAC for aranya
   $ kubectl create clusterrolebinding --clusterrole=aranya --serviceaccount=${NS}:aranya
   # deploy aranya to your cluster
   $ kubectl -n ${NS} apply -f https://github.com/arhat-dev/aranya/blob/master/cicd/k8s/aranya-deploy.yaml
   ```

2. Create a `EdgeDevice` resource object for each one of your edge devices (see [cicd/k8s/sample/example-edge-devices.yaml](./cicd/k8s/sample/example-edge-devices.yaml))
   1. `aranya` will create a `Kuberntes` node object according to the edge device spec in cluster
   2. connectivity between `aranya` and your edge devices
      1. `aranya` would create a `Kubenetes` service object if the edge device uses grpc to connect
      2. `aranya` would connect to your mqtt broker if the edge device is connected via MQTT
3. Create `Kubernetes` workloads with special taints (see [cicd/k8s/sample/example-workload.yaml](./cicd/k8s/sample/example-workload.yaml))
