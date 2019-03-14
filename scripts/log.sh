#!/bin/bash

POD_NAME=$(kubectl get pods -n edge -o json | jq -r '.items[].metadata.name' | grep 'aranya')

kubectl logs -n edge ${POD_NAME} -f \
    | jq -R -c -r '. as $line | try fromjson catch $line | {t: .ts | strflocaltime("%Y-%m-%d %H:%M:%S %Z"), c:.logger, msg:.msg}' \
    1>&2
