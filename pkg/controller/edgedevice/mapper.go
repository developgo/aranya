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

	name := types.NamespacedName{Namespace: i.Meta.GetNamespace(), Name: ownerRef.Name}
	svcLog.Info("found edge device for service")
	return []reconcile.Request{{NamespacedName: name}}
}
