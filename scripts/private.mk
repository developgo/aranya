NS := edge

.PHONY: ingress-setup
ingress-setup:
	kubectl -n ${NS} apply -f cicd/k8s/sample/ingress.yaml

.PHONY: ingress-cleanup
ingress-cleanup:
	kubectl -n ${NS} delete -f cicd/k8s/sample/ingress.yaml

.PHONY: run-arhat-fake-grpc
run-arhat-fake-grpc: arhat-fake-grpc
	./$(BUILD_DIR)/arhat-fake-grpc -c config/arhat/private_fake_grpc.yaml 1>&2

.PHONY: run-arhat-docker-grpc
run-arhat-docker-grpc: arhat-docker-grpc
	./$(BUILD_DIR)/arhat-docker-grpc -c config/arhat/private_docker_grpc.yaml 1>&2
