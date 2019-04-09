NS := edge

.PHONY: operator-setup
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

.PHONY: operator-cleanup
operator-cleanup: delete-sample-devices
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

.PHONY: deploy-sample-devices
deploy-sample-devices:
	kubectl -n ${NS} apply -f cicd/k8s/sample/sample-edge-devices.yaml

.PHONY: delete-sample-devices
delete-sample-devices:
	kubectl -n ${NS} delete -f cicd/k8s/sample/sample-edge-devices.yaml || true

.PHONY: deploy-sample-workload
deploy-sample-workload:
	kubectl -n ${NS} apply -f cicd/k8s/sample/sample-workload.yaml

.PHONY: delete-sample-workload
delete-sample-workload:
	kubectl -n ${NS} delete -f cicd/k8s/sample/sample-workload.yaml || true
