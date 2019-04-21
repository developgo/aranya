#!/bin/bash

set -e

gen-deploy-script() {
  for i in $(seq 1 1 $1); do
  echo "---
apiVersion: aranya.arhat.dev/v1alpha1
kind: EdgeDevice
metadata:
  name: test-device-grpc-${i}
spec:
  connectivity:
    method: grpc
    grpcConfig:
      tlsSecretRef: {}"
  done
}

gen-deploy-script $@
