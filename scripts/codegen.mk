SDK := operator-sdk

.PHONY: codegen
codegen:
	$(SDK) generate k8s
	$(SDK) generate openapi

.PHONY: proto_gen
proto_gen:
	$(shell scripts/pb.sh)
	@echo "proto files generated"
