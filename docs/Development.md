# Development

## Components

- `aranya` (`阿兰若`)
  - Role: The `Kubernetes` controller to provision virtual node and manage edge device
  - Origin: `aranya` is the remote place where `Sangha` do the spiritual practice (Sadhana).
    - `阿兰若` 是 `僧众` 修行的地方, 常位于远离人烟之处
- `arhat` (`阿罗汉`)
  - Role: The agent deployed at your edge device, finally will communicate with `aranya`
  - Origin: `arhat` is the man whose practice level is just next to `Buddha`
    - `阿罗汉` 代称 `僧众` 中取得了仅次于 `佛` 的果位的修行者

## Concepts

- Virtual Node
  - The node managed by `aranya`, act as a `Kubernetes` node, but only maintains status
- `EdgeDevice`
  - A resource type defined by `aranya`'s `Kubernetes Custom Resource Definition`

## Convensions

### Naming

- Kubernetes Resource Objects
  - `{foo}Obj`, `{foo}Object`
