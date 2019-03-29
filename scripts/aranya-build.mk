include scripts/tools.mk

ARANYA_SRC := ./cmd/aranya

.PHONY: aranya
aranya:
	CGO_ENABLED=0 \
	$(GOBUILD) \
		-o build/$@ \
		$(ARANYA_SRC)
