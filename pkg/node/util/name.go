package util

func GetFullPodName(namespace, name string) string {
	return namespace + "-" + name
}
