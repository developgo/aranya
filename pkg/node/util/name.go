package util

func GetFullPodName(namespace, name string) string {
	return namespace + "-" + name
}

func GetVirtualNodeName(deviceName string) string {
	return "aranya-node-for-" + deviceName
}

func GetServiceName(deviceName string) string {
	return "aranya-svc-for-" + deviceName
}
