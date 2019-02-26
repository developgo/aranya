#!/bin/bash

set -ex

gen_service_protos() {
  local TARGET_DIR=./pkg/node/service
  protoc -I${TARGET_DIR} \
    --go_out=plugins=grpc:${TARGET_DIR} \
    ${TARGET_DIR}/*.proto
}

gen_service_protos
