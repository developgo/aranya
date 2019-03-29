GOBUILD := GO111MODULE=on go build -mod=vendor
GOTEST := CGO_ENABLED=1 GO111MODULE=on go test -v -race -mod=vendor
