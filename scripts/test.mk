include scripts/tools.mk

test:
	$(GOTEST) \
	./pkg/node/manager \
	./pkg/node/agent \
	./pkg/node/pod/queue \
	./pkg/node/pod/cache
