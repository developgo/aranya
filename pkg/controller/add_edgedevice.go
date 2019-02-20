package controller

import (
	"arhat.dev/aranya/pkg/controller/edgedevice"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, edgedevice.Add)
}
