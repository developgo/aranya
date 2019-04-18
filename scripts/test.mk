include scripts/tools.mk

test:
	$(GOTEST) \
	./pkg/node/manager \
	./pkg/node/agent