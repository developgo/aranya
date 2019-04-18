package runtimeutil

import (
	"crypto/sha256"
	"encoding/hex"
)

func GetContainerName(podUID, container string) string {
	raw := podUID + "/" + container
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
