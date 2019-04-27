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
   - Use `StatefulSet` or `nodeAffinity` to avoid unexpected certification regenation when `aranya` is deployed to different nodes.
   - Changes to `Node`'s address(es) or hostname(s) (which is the rare case) when `aranya` has been serving the `kubelet` servers and connectivity managers for edge devices would result in connectivity failure and remote management failure, you need to restart `aranya` to solve this problem.

## Host Management

`Kubernetes` doesn't allow users to directly control the node via `kubectl`, but with the help of privileged `Pod`, we can do some host maintenance work. (Check out my device plugin [arhat-dev/kube-host-pty](https://github.com/arhat-dev/kube-host-pty) to enable host access via `kubectl` if you are interested)

I would say, it's the best practice for cloud computing to isolate maintenance and deployment with such barrier, but it's not so good in the case of edge devices, because in this way:

If using `ssh` (or `ansible`) for device management (which enables full management):

- Edge devices need to expose ssh service port for remote management
  - Most attack to IoT network happens to ssh services
  - SSH key management in remote devices is another big problem
  - Connectivity through network with NAT and Firewall requires extra effort (such as `frp` service)

If using privileged `Pod` for management:

- We need to maintain a set of management container images
  - To download when needed
    - data usage is a thing
  - To store preflight
    - storage may be a concern

So what if we need to be able to access our edge device at any place and at any time, once the edge device has been online?

`aranya` and `arhat` solves this problem with the concept `virtual pod`

`virtual pod` is a `Pod` object only living in `Kubernetes` cluster and never will be deployed to your edge devices, its phase is always `Running`. Any `kubectl` command to the `virtual pod` such as `logs`, `exec`, `attach` and `port-forward` will be treated as works to do in device host.

No ssh service exposure, no actual `Pod` deployment, maintenance work done right for edge devices ;)
