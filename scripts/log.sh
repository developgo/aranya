#!/bin/bash

POD_NAME=$(kubectl get -n edge pods -o jsonpath='{.items[].metadata.name}')

current() {
    kubectl logs -n edge ${POD_NAME} -f \
        | jq -R -c -r '. as $line | try fromjson catch $line | {t: .ts | strflocaltime("%Y-%m-%d %H:%M:%S %Z"), c:.logger, msg:.msg, err: .error}' \
        1>&2
}

previous() {
    kubectl logs -n edge ${POD_NAME} -p \
        | jq -R -c -r '. as $line | try fromjson catch $line | {t: .ts | strflocaltime("%Y-%m-%d %H:%M:%S %Z"), c:.logger, msg:.msg, err: .error}' \
        1>&2
}

"$@"