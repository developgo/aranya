SDK := operator-sdk

.PHONY: install-codegen-tools
install-codegen-tools:
	./scripts/codegen.sh install-deepcopy-gen
	./scripts/codegen.sh install-openapi-gen

.PHONY: gen-code
gen-code:
	./scripts/codegen.sh gen-deepcopy
	./scripts/codegen.sh gen-openapi

.PHONY: gen-proto
gen-proto:
	./scripts/pb.sh
	@echo "proto files generated"
