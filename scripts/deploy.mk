NS := edge

.PHONY: setup
operator-setup:
	# ns
	kubectl create namespace ${NS} || true
	# crd
	kubectl apply -f cicd/k8s/crds/aranya_v1alpha1_edgedevice_crd.yaml
	# rbac
	kubectl apply -f cicd/k8s/aranya-cluster-role.yaml
	kubectl -n ${NS} create serviceaccount aranya || true
	kubectl create clusterrolebinding aranya \
		--clusterrole=aranya --serviceaccount=${NS}:aranya || true
	# deploy
	kubectl apply -n $(NS) -f cicd/k8s/aranya-deploy.yaml

.PHONY: cleanup
operator-cleanup: delete-sample
	# delete deployment
	kubectl delete -n $(NS) -f cicd/k8s/aranya-deploy.yaml || true
	# delete rbac
	kubectl delete clusterrolebinding aranya || true
	kubectl -n ${NS} delete serviceaccount aranya || true
	kubectl delete -f cicd/k8s/aranya-cluster-role.yaml || true
	# delete crd
	kubectl delete -f cicd/k8s/crds/aranya_v1alpha1_edgedevice_crd.yaml || true
	# delete ns
	kubectl delete namespace ${NS} || true

.PHONY: deploy-sample
deploy-sample:
	kubectl -n ${NS} apply -f cicd/k8s/sample/example-edge-devices.yaml

.PHONY: delete-sample
delete-sample:
	kubectl -n ${NS} delete -f cicd/k8s/sample/example-edge-devices.yaml
