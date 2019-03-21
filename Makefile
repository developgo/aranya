IMAGE_NAME := arhatdev/aranya:latest
DOCKER := docker
SDK := operator-sdk

NS := edge

.PHONY: aranya
aranya:
	CGO_ENABLED=0 GO111MODULE=on go build -mod=vendor -o build/aranya ./cmd/aranya

.PHONY: arhat
arhat-podman-grpc:
	CGO_ENABLED=1 GO111MODULE=on GOOS=linux go build -mod=vendor -tags='podman grpc' -o build/arhat ./cmd/arhat/

test:
	CGO_ENABLED=1 GO111MODULE=on go test -v -race -mod=vendor \
		./pkg/node/connectivity/client ./pkg/node/connectivity/server

.PHONY: build-image
build-image:
	$(DOCKER) build -t $(IMAGE_NAME) -f docker/aranya.dockerfile

push-image: build-image
	$(DOCKER) push $(IMAGE_NAME)

.PHONY: setup
setup:
	# ns
	kubectl create namespace ${NS} || true
	# crd
	kubectl apply -f cicd/k8s/crds/aranya_v1alpha1_edgedevice_crd.yaml
	# rbac
	kubectl apply -f cicd/k8s/aranya-cluster-role.yaml
	kubectl -n ${NS} create serviceaccount aranya || true
	kubectl create clusterrolebinding aranya \
		--clusterrole=aranya --serviceaccount=${NS}:aranya || true
	# deploy
	kubectl apply -n $(NS) -f cicd/k8s/aranya-deploy.yaml

.PHONY: cleanup
cleanup:
	# delete deployment
	kubectl delete -n $(NS) -f cicd/k8s/aranya-deploy.yaml || true
	# delete rbac
	kubectl delete clusterrolebinding aranya || true
	kubectl -n ${NS} delete serviceaccount aranya || true
	kubectl delete -f cicd/k8s/aranya-cluster-role.yaml || true
	# delete crd
	kubectl delete -f cicd/k8s/crds/aranya_v1alpha1_edgedevice_crd.yaml || true
	# delete ns
	kubectl delete namespace ${NS} || true

.PHONY: deploy-sample
deploy-sample:
	kubectl -n ${NS} apply -f cicd/k8s/sample/example-edge-devices.yaml

.PHONY: delete-sample
delete-sample:
	kubectl -n ${NS} delete -f cicd/k8s/sample/example-edge-devices.yaml

.PHONY: codegen
codegen:
	$(SDK) generate k8s
	$(SDK) generate openapi

.PHONY: proto_gen
proto_gen:
	$(shell scripts/pb.sh)
	@echo "proto files generated"

.PHONY: check_log
check_log:
	$(shell scripts/log.sh)
