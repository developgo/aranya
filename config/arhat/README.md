# `arhat` Config

Though there are multiple sample configurations, they follow the same configuration model and have same configuration fields.

## Details

- `agent` - defines `arhat`'s behavior
- `runtime` - defines the way to communicate with container runtime
  - different runtime has different requirements for configuration fields, please refer to these configuration samples for details
- `connectivity` - defines the way to communicate with `aranya`
  - requires its own configuration depending on the `arhat` type
    - `arhat-{}-grpc` requires `.connectivity.grpc_config`
    - `arhat-{}-mqtt` requires `.connectivity.mqtt_config`
