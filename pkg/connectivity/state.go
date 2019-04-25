/*
Copyright 2019 The arhat.dev Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
