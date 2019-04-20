package runtimeutil

import (
	"fmt"
)

func GetContainerName(namespace, name, container string) string {
	return fmt.Sprintf("%s.%s.%s", namespace, name, container)
}
