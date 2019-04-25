# Maintenance

## Maintenance Model

`Kubernetes` defines the concept `virtual cluster` via the `namespace` resource type, we will take the advantage of that concept for our maintenance.

1. Both `aranya`'s deployment and `EdgeDevice`s are namespaced (see [Behaviors and Tips](#behaviors-and-tips) for more information).
2. `Node` objects in `Kubernetes` are not namespaced but those managed by `aranya` will contain a taint for `namespace` name.
   - You need to create workloads with tolerations if you do expect the workload to run in edge devices.

## Behaviors and Tips

1. `aranya` watches the `namespace` it has been deployed to, reconciles `EdgeDevice`s created in the same `namespace`.
   - You can deploy `aranya` to watch some `namespace` other than the one it was deployed to, but this is strongly discouraged for maintenance reason.
2. Only one `aranya` instance in the same `namespace` will work as leader to do the reconcile job and serve `kubelet` servers and connectivity managers.
   - Deploy `aranya` to mutiple `namespace`s if you have quite a lot (tipically more than 500) `EdgeDevice`s to deploy and group the `EdgeDevice`s into different `namespace`s.
3. `aranya` requires host network to work properly.
   - Deploy multiple `aranya` in the same `namespace` to different `Node`s to avoid single point failure.
4. `aranya` will request node certifications for each one of the `EdgeDevice`s you have deployed. The node certification includes the `Node`'s address(es) and hostname(s) (Here the `Node` is the one `aranya` deployed to).
   - Use `DaemonSet` or `nodeAffinity` to avoid unexpected certification regenation when `aranya` is deployed to different nodes.
   - Changes to `Node`'s address(es) or hostname(s) (which is the rare case) when `aranya` has been serving the `kubelet` servers and connectivity managers for edge devices would result in connectivity failure and remote management failure, you need to restart `aranya` to solve this problem.
