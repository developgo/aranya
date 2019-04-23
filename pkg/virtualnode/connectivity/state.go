package connectivity

func (s *PodStatus_ContainerStatus) GetState() PodStatus_State {
	if s == nil {
		return StateUnknown
	}

	if s.FinishedAt > 0 {
		if s.ExitCode != 0 {
			return StateFailed
		} else {
			return StateSucceeded
		}
	}

	if s.StartedAt > 0 {
		return StateRunning
	}

	if s.CreatedAt > 0 {
		return StatePending
	}

	if s.ContainerId == "" {
		return StateUnknown
	}

	return StatePending
}
