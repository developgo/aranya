package connectivity

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

func (m *Node) GetResolvedCoreV1Node() (*corev1.Node, error) {
	node := &corev1.Node{}

	if m.GetNodeV1() == nil {
		return nil, fmt.Errorf("no core v1 node info present")
	}

	err := node.Unmarshal(m.GetNodeV1())
	if err != nil {
		return nil, err
	}

	return node, nil
}
