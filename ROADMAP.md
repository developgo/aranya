# Roadmap

## Networking

- Integrate edge devices into cluster network
  - `Kubernetes` will assign a pod address pool for the virtual node
  - `arhat` to create address mappings between local address and cluster address and DNS records to redirect local network traffic to Kubernetes cluster, so we can improve the cloud secuirty by accessing cloud services without `Ingress` or any thing exposed to public Internet.
  - `aranya` to create userspace proxier (`kube-proxy`) to proxy specific cloud traffics for pods and services to `aranya` and finally to `arhat`, abstracting away the underlaying protocol between `aranya` and `arhat`, also enabling `kubectl proxy` commands to proxy http services running on edge devices.
