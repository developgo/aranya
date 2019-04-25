/*
Copyright 2019 The arhat.dev Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package edgedevice

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ServiceMapper struct {
	ctx    context.Context
	client client.Client
}

func (m *ServiceMapper) Map(i handler.MapObject) []reconcile.Request {
	if i.Meta == nil {
		return nil
	}

	svcLog := log.WithValues("svc.name", i.Meta.GetName(), "svc.namespace", i.Meta.GetNamespace())
	svcLog.Info("attempt to map service to edge device")
	ownerRef := metav1.GetControllerOf(i.Meta)
	if ownerRef == nil || ownerRef.Kind != "EdgeDevice" {
		return nil
	}

	svcLog.Info("found edge device for service")
	return []reconcile.Request{{
		NamespacedName: types.NamespacedName{
			Namespace: i.Meta.GetNamespace(),
			Name:      ownerRef.Name,
		},
	}}
}
