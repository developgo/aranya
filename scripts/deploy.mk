# Copyright 2019 The arhat.dev Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

NS := edge

KUBE_CREATE := kubectl create -n ${NS}
KUBE_APPLY := kubectl apply -n ${NS}
KUBE_DEL := kubectl delete -n ${NS}

.PHONY: operator-setup
operator-setup:
	# ns
	kubectl create namespace ${NS} || true
	# crd
	${KUBE_APPLY} -f cicd/k8s/crds/aranya_v1alpha1_edgedevice_crd.yaml
	# rbac
	${KUBE_APPLY} -f cicd/k8s/aranya-roles.yaml
	${KUBE_CREATE} serviceaccount aranya || true
	${KUBE_CREATE} rolebinding aranya --role=aranya --serviceaccount=${NS}:aranya || true
	kubectl create clusterrolebinding aranya --clusterrole=aranya --serviceaccount=${NS}:aranya || true
	# deploy
	${KUBE_APPLY} -f cicd/k8s/aranya-deploy.yaml

.PHONY: operator-cleanup
operator-cleanup: delete-sample-devices
	# delete deployment
	${KUBE_DEL} -f cicd/k8s/aranya-deploy.yaml || true
	# delete rbac
	${KUBE_DEL} rolebinding aranya || true
	${KUBE_DEL} clusterrolebinding aranya || true
	${KUBE_DEL} serviceaccount aranya || true
	${KUBE_DEL} -f cicd/k8s/aranya-cluster-role.yaml || true
	# delete crd
	${KUBE_DEL} -f cicd/k8s/crds/aranya_v1alpha1_edgedevice_crd.yaml || true
	# delete ns
	${KUBE_DEL} namespace ${NS} || true

.PHONY: deploy-sample-devices
deploy-sample-devices:
	${KUBE_APPLY} -f cicd/k8s/sample/sample-edge-devices.yaml

.PHONY: delete-sample-devices
delete-sample-devices:
	${KUBE_DEL} -f cicd/k8s/sample/sample-edge-devices.yaml || true

.PHONY: deploy-sample-workload
deploy-sample-workload:
	${KUBE_APPLY} -f cicd/k8s/sample/sample-workload.yaml

.PHONY: delete-sample-workload
delete-sample-workload:
	${KUBE_DEL} -f cicd/k8s/sample/sample-workload.yaml || true
