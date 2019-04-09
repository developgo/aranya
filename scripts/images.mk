DOCKER := docker
IMAGE_NAME := arhatdev/aranya:latest

.PHONY: build-image
build-image:
	$(DOCKER) build -t $(IMAGE_NAME) -f docker/aranya.dockerfile

.PHONY: push-image
push-image: build-image
	$(DOCKER) push $(IMAGE_NAME)
