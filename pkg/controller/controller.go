package controller

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"arhat.dev/aranya/pkg/controller/edgedevice"
)

// AddToManager adds all Controllers to the Manager
func AddToManager(m manager.Manager) error {
	controllerAdders := []func(manager.Manager) error{
		edgedevice.AddToManager,
	}

	for _, addTo := range controllerAdders {
		if err := addTo(m); err != nil {
			return err
		}
	}
	return nil
}
