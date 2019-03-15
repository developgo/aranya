#!/bin/bash

POD_NAME=$(kubectl get -n edge pods -o jsonpath='{.items[].metadata.name}')

kubectl logs -n edge ${POD_NAME} -f \
    | jq -R -c -r '. as $line | try fromjson catch $line | {t: .ts | strflocaltime("%Y-%m-%d %H:%M:%S %Z"), c:.logger, msg:.msg, err: .error}' \
    1>&2
