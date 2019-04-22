include scripts/tools.mk

test:
	$(GOTEST) \
	./pkg/virtualnode/manager \
	./pkg/virtualnode/agent \
	./pkg/virtualnode/pod/queue \
	./pkg/virtualnode/pod/cache
