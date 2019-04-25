# Build

## Prerequisites

- Just to build
  - `go` 1.11+ (to support gomodule)
  - `git` (to clone this project)
  - `make` (to ease you life with `aranya` development)
- Need to update CRDs
  - __+__ `GOPATH` configured
    - install code generators with `make install-codegen-tools`
- Need to update connectivity api
  - __+__ `protoc` 3.5+ (protobuf compiler)

## Before you start

1. This porject's gomodule name is `arhat.dev/aranya`
2. Clone this project from github

```bash
git clone https://github.com/arhat-dev/aranya

# or if you have to, use go get (discouraged)
# $ go get -u arhat.dev/aranya
```

## Build Instructions

### Build aranya

```bash
# build the binary directly
make aranya

# build in docker image (no binary output required)
make build-image-aranya
```

### Build `arhat`

`arhat` tagets are named in format of `arhat-{runtime-name}-{connectivity-method}`

Avaliable `arhat` targets:

- `arhat-docker-grpc`
- `arhat-docker-mqtt`
- `arhat-containerd-grpc`
- `arhat-containerd-mqtt`
- `arhat-podman-grpc`
- `arhat-podman-mqtt`
- `arhat-cri-grpc`
- `arhat-cri-mqtt`

Example:

```bash
# build the binary directly
make arhat-docker-grpc

# build in docker image (no binary output required)
make build-image-arhat-docker-grpc
# to run arhat in container
# $ docker run -d \
#       -v /var/run/docker.sock:/var/run/docker.sock:ro \
#       -v /etc/arhat/config.yaml:$(pwd)/config/arhat/sample-docker.yaml:ro \
#       arhatdev/arhat-docker-grpc:latest /app -c /etc/arhat/config.yaml
```

### Generate Protobuf files

```bash
make gen-proto
```

### Generate Code for Kubernetes CRDs

```bash
make gen-code
```

__Known Issue__: currently openapi specification will not be updated.
