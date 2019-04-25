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

include scripts/aranya-build.mk
include scripts/arhat-build.mk

include scripts/test.mk
include scripts/images.mk
include scripts/deploy.mk
include scripts/codegen.mk

.PHONY: check-log
check-log:
	$(shell scripts/log.sh current)

.PHONY: check-log-all
check-log-all:
	$(shell scripts/log.sh current-all)

.PHONY: check-log-prev
check-log-prev:
	$(shell scripts/log.sh previous)

-include private/scripts.mk
