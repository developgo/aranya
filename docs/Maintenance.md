# Maintenance

## Maintenance Model

`Kubernetes` defines the concept `virtual cluster` via the `namespace` resource type, we will facilitate that concept for our maintenance model.

1. Every `aranya` belongs to a specific `virtual cluster`
2. Virtual nodes created by `aranya` will contain a taint for the `virtual cluster` by default
