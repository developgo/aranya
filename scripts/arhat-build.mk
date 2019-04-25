# Copyright 2019 The arhat.dev Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

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
