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

check-log-prev:
	$(shell scripts/log.sh previous)
