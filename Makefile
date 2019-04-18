include scripts/aranya-build.mk
include scripts/arhat-build.mk

include scripts/test.mk
include scripts/images.mk
include scripts/deploy.mk
include scripts/codegen.mk

include scripts/private.mk

.PHONY: check-log
check-log:
	$(shell scripts/log.sh current)

.PHONY: check-log-all
check-log-all:
	$(shell scripts/log.sh current-all)

.PHONY: check-log-prev
check-log-prev:
	$(shell scripts/log.sh previous)
