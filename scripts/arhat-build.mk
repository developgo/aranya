include scripts/tools.mk

BUILD_DIR := build
ARHAT_SRC := ./cmd/arhat

# target name format:
# 	arhat-{runtime}-{connectivity}


#
# Connect via gRPC
#

.PHONY: arhat-podman-grpc
arhat-podman-grpc:
	CGO_ENABLED=1 GOOS=linux\
	$(GOBUILD) \
		-tags='rt_podman agent_grpc' \
		-o $(BUILD_DIR)/$@ \
		$(ARHAT_SRC)

.PHONY: arhat-cri-grpc
arhat-cri-grpc:
	CGO_ENABLED=0 \
	$(GOBUILD) \
		-tags='rt_cri agent_grpc' \
		-o $(BUILD_DIR)/$@ \
		$(ARHAT_SRC)

.PHONY: arhat-fake-grpc
arhat-fake-grpc:
	CGO_ENABLED=0 \
	$(GOBUILD) \
		-tags='rt_fake agent_grpc' \
		-o $(BUILD_DIR)/$@ \
		$(ARHAT_SRC)

.PHONY: arhat-containerd-grpc
arhat-containerd-grpc:
	CGO_ENABLED=0 \
	$(GOBUILD) \
		-tags='rt_containerd agent_grpc' \
		-o $(BUILD_DIR)/$@ \
		$(ARHAT_SRC)

.PHONY: arhat-docker-grpc
arhat-docker-grpc:
	CGO_ENABLED=0 \
	$(GOBUILD) \
		-tags='rt_docker agent_grpc' \
		-o $(BUILD_DIR)/$@ \
		$(ARHAT_SRC)

#
# Connect via MQTT
#

.PHONY: arhat-podman-mqtt
arhat-podman-mqtt:
	CGO_ENABLED=1 GOOS=linux \
	$(GOBUILD) \
		-tags='rt_podman agent_mqtt' \
		-o $(BUILD_DIR)/$@ \
		$(ARHAT_SRC)

.PHONY: arhat-cri-mqtt
arhat-cri-mqtt:
	CGO_ENABLED=0 \
	$(GOBUILD) \
		-tags='rt_cri agent_mqtt' \
		-o $(BUILD_DIR)/$@ \
		$(ARHAT_SRC)

.PHONY: arhat-fake-mqtt
arhat-fake-mqtt:
	CGO_ENABLED=0 \
	$(GOBUILD) \
		-tags='rt_fake agent_mqtt' \
		-o $(BUILD_DIR)/$@ \
		$(ARHAT_SRC)

.PHONY: arhat-containerd-mqtt
arhat-containerd-mqtt:
	CGO_ENABLED=0 \
	$(GOBUILD) \
		-tags='rt_containerd agent_mqtt' \
		-o $(BUILD_DIR)/$@ \
		$(ARHAT_SRC)

.PHONY: arhat-docker-mqtt
arhat-docker-mqtt:
	CGO_ENABLED=0 \
	$(GOBUILD) \
		-tags='rt_docker agent_mqtt' \
		-o $(BUILD_DIR)/$@ \
		$(ARHAT_SRC)
