SDK := operator-sdk

.PHONY: gen-code
gen-code:
	$(SDK) generate k8s
	$(SDK) generate openapi

.PHONY: gen-proto
gen-proto:
	$(shell scripts/pb.sh)
	@echo "proto files generated"
