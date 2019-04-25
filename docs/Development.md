# Development

## Components

- `aranya` (`阿兰若`)
  - Role: The `Kubernetes` controller to provision virtual node and manage edge device
  - Origin: `aranya` is the remote place where `sangha` do the spiritual practice (sadhana).
    - `阿兰若` 是 `僧众` 修行的地方, 常位于远离人烟之处
- `arhat` (`阿罗汉`)
  - Role: The agent deployed at your edge device, communicate with `aranya` (via `gRPC` or message brokers)
  - Origin: `arhat` is the man whose sadhana level is just next to `buddha`
    - `阿罗汉` 取得了仅次于 `佛` 果位的修行者

## Concepts

- `EdgeDevice`
  - A resource type defined by `aranya`'s `Kubernetes Custom Resource Definition`
- edge device
  - A physical computer/device with contaienr runtime (e.g. raspberry pi with docker installed)
- virtualnode
  - The node managed by `aranya`, act as a `Kubernetes` `Node`
  - Functions:
    - Sync `Node`/`Pod`s status with `Kubernetes` master and edge device.
    - Schedule `Pod`s to edge devices.
    - `kubelet` server for remote management.
    - Connectivity manager to handle edge device connection.
