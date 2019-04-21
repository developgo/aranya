#!/bin/bash -x

set -e

export GOPATH=$(go env GOPATH)

OPENAPI_GEN="${GOPATH}/bin/kube-openapi-gen"
DEEPCOPY_GEN="${GOPATH}/bin/kube-deepcopy-gen"

download-pakage() {
    GO111MODULE=off go get -v -d $1
}

install-deepcopy-gen() {
    download-pakage k8s.io/code-generator/cmd/deepcopy-gen

    pushd "${GOPATH}/src/k8s.io/code-generator"
    GO111MODULE=on go build -o ${DEEPCOPY_GEN} ./cmd/deepcopy-gen/
    popd
}

install-openapi-gen() {
    download-pakage k8s.io/kube-openapi/cmd/openapi-gen

    pushd "${GOPATH}/src/k8s.io/kube-openapi"
    GO111MODULE=on go build -o ${OPENAPI_GEN} ./cmd/openapi-gen/
    popd
}

gen-deepcopy() {
    ${GOPATH}/src/k8s.io/code-generator/generate-groups.sh deepcopy \
        - \
        ./pkg/apis "aranya:v1alpha1" \
        --go-header-file $(pwd)/scripts/boilerplate.go.txt \
        -v 2

    mv "${GOPATH}/src/pkg/apis/aranya/v1alpha1/zz_generated.deepcopy.go" \
        ./pkg/apis/aranya/v1alpha1/zz_generated.deepcopy.go
}

gen-openapi() {
    ${OPENAPI_GEN} \
        --input-dirs ./pkg/apis/aranya/v1alpha1/ \
        --output-package ./pkg/aranya/v1alpha1/ \
        --go-header-file ./scripts/boilerplate.go.txt \
        --output-file-base zz_generated.openapi \
        -v 2

    mv "${GOPATH}/src/pkg/aranya/v1alpha1/zz_generated.openapi.go" \
        ./pkg/apis/aranya/v1alpha1/zz_generated.openapi.go
}

"$@"
