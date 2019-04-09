NS := edge

.PHONY: ingress-setup
ingress-setup:
	kubectl -n ${NS} apply -f cicd/k8s/sample/ingress.yaml

.PHONY: ingress-cleanup
ingress-cleanup:
	kubectl -n ${NS} delete -f cicd/k8s/sample/ingress.yaml

.PHONY: run-arhat
run-arhat-fake-grpc: arhat-fake-grpc
	./$(BUILD_DIR)/arhat-fake-grpc -c config/arhat/private_fake_grpc.yaml
