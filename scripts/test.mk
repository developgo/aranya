include scripts/tools.mk

test:
	$(GOTEST) \
		./pkg/node/connectivity/server \
		./pkg/node/connectivity/client