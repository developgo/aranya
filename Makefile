IMAGE_NAME := arhatdev/aranya:latest
DOCKER := docker
SDK := operator-sdk

PROJECT_GOPATH := $(GOPATH)/src/arhat.dev/aranya

.PHONY: aranya
aranya:
	CGO_ENABLED=0 GO111MODULE=on go build -mod=vendor -o build/aranya cmd/manager/*.go

.PHONY: build-image
build-image:
	$(DOCKER) build -t $(IMAGE_NAME) -f docker/aranya.dockerfile

push-image: build-image
	$(DOCKER) push $(IMAGE_NAME)

.PHONY: setup
setup:
	kubectl apply -f cicd/k8s/crds/aranya_v1alpha1_edgedevice_crd.yaml
	kubectl apply -f cicd/k8s/aranya.yaml

.PHONY: cleanup
cleanup: delete-sample
	kubectl delete -f cicd/k8s/aranya.yaml
	kubectl delete -f cicd/k8s/crds/aranya_v1alpha1_edgedevice_crd.yaml

.PHONY: deploy-sample
deploy-sample:
	kubectl apply -f cicd/k8s/sample/example-edge-devices.yaml

.PHONY: delete-sample
delete-sample:
	kubectl delete -f cicd/k8s/sample/example-edge-devices.yaml

.PHONY: codegen
codegen:
	$(SDK) generate k8s
	$(SDK) generate openapi
