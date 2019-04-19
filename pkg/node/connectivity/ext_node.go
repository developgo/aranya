package connectivity

import (
	"errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var (
	ErrNotResolved = errors.New("no field to be resolved")
)

func (m *Node) HasSystemInfo() bool {
	if m.GetSystemInfo() == nil || len(m.GetSystemInfo()) == 0 {
		return false
	}
	return true
}

func (m *Node) GetResolvedSystemInfo() (*corev1.NodeSystemInfo, error) {
	if !m.HasSystemInfo() {
		return nil, ErrNotResolved
	}

	info := &corev1.NodeSystemInfo{}
	if err := info.Unmarshal(m.GetSystemInfo()); err != nil {
		return nil, err
	}
	return info, nil
}

func (m *Node) HasCapacity() bool {
	if m.GetResources() == nil || len(m.GetResources().GetCapacity()) == 0 {
		return false
	}
	return true
}

func (m *Node) GetResolvedCapacity() (corev1.ResourceList, error) {
	if !m.HasCapacity() {
		return nil, ErrNotResolved
	}

	capacity := make(corev1.ResourceList)
	for name, quantityBytes := range m.GetResources().GetCapacity() {
		quantity := resource.Quantity{}
		if err := (&quantity).Unmarshal(quantityBytes); err != nil {
			return nil, err
		}

		capacity[corev1.ResourceName(name)] = quantity
	}

	return capacity, nil
}

func (m *Node) HasAllocatable() bool {
	if m.GetResources() == nil || len(m.GetResources().GetAllocatable()) == 0 {
		return false
	}
	return true
}

func (m *Node) GetResolvedAllocatable() (corev1.ResourceList, error) {
	if !m.HasAllocatable() {
		return nil, ErrNotResolved
	}

	allocatable := make(corev1.ResourceList)
	for name, quantityBytes := range m.GetResources().GetCapacity() {
		quantity := resource.Quantity{}
		if err := (&quantity).Unmarshal(quantityBytes); err != nil {
			return nil, err
		}

		allocatable[corev1.ResourceName(name)] = quantity
	}

	return allocatable, nil
}

func (m *Node) HasConditions() bool {
	if m.GetConditions() == nil || len(m.GetConditions().GetConditions()) == 0 {
		return false
	}

	return true
}

func (m *Node) GetResolvedConditions() ([]corev1.NodeCondition, error) {
	if !m.HasConditions() {
		return nil, ErrNotResolved
	}

	var conds []corev1.NodeCondition
	for _, condBytes := range m.GetConditions().GetConditions() {
		cond := corev1.NodeCondition{}
		if err := (&cond).Unmarshal(condBytes); err != nil {
			return nil, err
		}

		conds = append(conds, cond)
	}
	return conds, nil
}
