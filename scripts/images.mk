# alternative: podman
DOCKER := docker
DOCKERBUILD := $(DOCKER) build -f cicd/docker/app.dockerfile

ARANYA_IMAGE := arhatdev/aranya:latest
ARHAT_DOCKER_GRPC_IMAGE := arhatdev/arhat-docker-grpc:latest

.PHONY: build-image-aranya
build-image-aranya:
	$(DOCKERBUILD) --build-arg TARGET=aranya -t $(ARANYA_IMAGE) .

.PHONY: push-image-aranya
push-image-aranya-image:
	$(DOCKER) push $(ARANYA_IMAGE)

.PHONY: build-image-arhat-docker-grpc
build-image-arhat-docker-grpc:
	$(DOCKERBUILD) --build-arg TARGET=arhat-docker-grpc -t $(ARHAT_DOCKER_GRPC_IMAGE) .

.PHONY: push-image-arhat-docker-grpc
push-image-arhat-docker-grpc:
	$(DOCKER) push $(ARHAT_DOCKER_GRPC_IMAGE)
