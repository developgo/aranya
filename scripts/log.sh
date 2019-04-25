#!/bin/bash

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

current-all () {
    kubectl logs -n edge ${POD_NAME} -f \
        | jq -R -c '. as $line | try fromjson catch $line' \
        1>&2
}

"$@"
