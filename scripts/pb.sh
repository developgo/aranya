#!/bin/bash

set -ex

GOPATH=$(go env GOPATH)

gen_connectivity_protos() {
  local TARGET_DIR=./pkg/node/connectivity
  
  protoc \
    -I${GOPATH}/src/github.com/gogo/protobuf/protobuf \
    -I${GOPATH}/src \
    -I${TARGET_DIR} \
    --gogoslick_out=plugins=grpc:${TARGET_DIR} \
    ${TARGET_DIR}/*.proto
}

gen_connectivity_protos
