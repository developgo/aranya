package util

func GetFullPodName(namespace, name string) string {
	return namespace + "/" + name
}

func GetNodeName(namespace, name string) string {
	return namespace + "/" + name
}
