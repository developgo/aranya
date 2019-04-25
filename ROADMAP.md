# Roadmap

## Networking

- Integrate edge devices into cluster network (working on)
  - `Kubernetes` will assign a pod address pool for the virtual node
  - `arhat`'role
    - Create DNS/hosts records for `Kubernetes` services in edge devices.
    - Create network listeners for these services.
    - Create an iptables/ipvs based proxy to redirect local network traffic to `arhat` listeners.
    - Redirect data received from listeners to `aranya`.
  - `aranya`'s role
    - Receive application data transmitted by `arhat`.
    - Find and establish network connection(s) to the in cluster services, redirect the application traffic to it.
  - Then we can:
    - Improve the cloud secuirty by accessing cloud services without any thing in cluster exposed to public Internet.
    - Improve the robustness of edge devices by always using DNS names.

## Connectivity

- `gRPC` (already supported)
- `MQTT` (working on)
  - MQTT is the protocol designed to save IoT devices from synchronized TCP communications with a pub/sub model, this model presents new challenge to us for realtime sequential communication such as shell execution in remote edge device.
- `CoAP` (no plan for now)
  - Due to the same reason for MQTT, even worse with UDP.
- `Cap'n Proto (RPC)` (no plan for now)
  - no schedule for now

## Container Runtime

- `docker` (already supported)
  - `docker` is the most common container runtime, most developer familiar them with Linux container for the first time thanks to the help of `docker`
  - `Docker CE` is available for multiple system platforms and computer archs, including `Linux arm/arm64` and `macOS`, which is nice if you want to get `arhat` up and running without any deep dive into container world.
- `containerd` (working on)
  - `containerd` is the real `docker` runtime do container related jobs, it's light weighted.
  - `containerd`'s official release currently lacks multi-platform builds.
- `podman` (working on)
  - `podman` is the container runtime maintained by `RedHat`, with `Pod` model which is designed by `CoreOS`
  - `podman` can be embedded into `arhat` but only `Linux` host will be supported
- `cri` (no plan for now)
  - `cri` represents `Container Runtime Interface` which is the container runtime api definition designed for `Kubernetes`
