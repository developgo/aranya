#!/bin/bash

set -ex

gen_connectivity_protos() {
  local TARGET_DIR=./pkg/node/connectivity
  protoc -I${TARGET_DIR} \
    --go_out=plugins=grpc:${TARGET_DIR} \
    ${TARGET_DIR}/*.proto
}

gen_connectivity_protos
