GOBUILD := GO111MODULE=on go build -mod=vendor
GOTEST := CGO_ENABLED=1 GO111MODULE=on \
	go test -v -race -mod=vendor \
	-tags='agent_grpc agent_mqtt rt_docker rt_containerd rt_cri'
