include scripts/aranya-build.mk
include scripts/arhat-build.mk

include scripts/test.mk
include scripts/images.mk
include scripts/deploy.mk
include scripts/codegen.mk

.PHONY: check_log
check_log:
	$(shell scripts/log.sh)
