# Roadmap

## Work Mode

- Standalone Mode
  - A `aranya` living outside of `Kubernetes`
  - Good for small scale deployment

## Network

- Integrate into cluster network
  - `Kubernetes` will assign a pod address pool for the virtual node
  - `arhat` to create an address mapping between local address and cluster address
  - `arhat` to create DNS record to redirect local network traffic to cloud
  - access cloud services without `Ingress` or any thing like that
