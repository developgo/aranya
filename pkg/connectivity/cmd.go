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

import (
	"time"
)

func NewNodeCmd(action NodeCmd_Action) *Cmd {
	return &Cmd{
		Cmd: &Cmd_Node{
			Node: &NodeCmd{
				Action: action,
			},
		},
	}
}

func newPodCmd(sid uint64, action PodCmd_Action, options isPodCmd_Options) *Cmd {
	return &Cmd{
		SessionId: sid,
		Cmd: &Cmd_Pod{
			Pod: &PodCmd{
				Action:  action,
				Options: options,
			},
		},
	}
}

func NewPodCreateCmd(options *CreateOptions) *Cmd {
	return newPodCmd(0, CreatePod, &PodCmd_CreateOptions{
		CreateOptions: options,
	})
}

func NewPodDeleteCmd(podUID string, graceTime time.Duration) *Cmd {
	return newPodCmd(0, DeletePod, &PodCmd_DeleteOptions{
		DeleteOptions: &DeleteOptions{
			PodUid:    podUID,
			GraceTime: int64(graceTime),
		},
	})
}

func NewPodListCmd(namespace, name string, all bool) *Cmd {
	return newPodCmd(0, ListPods, &PodCmd_ListOptions{
		ListOptions: &ListOptions{
			Namespace: namespace,
			Name:      name,
			All:       all,
		},
	})
}

func NewContainerExecCmd(podUID, container string, command []string, stdin, stdout, stderr, tty bool) *Cmd {
	return newPodCmd(0, Exec, &PodCmd_ExecOptions{
		ExecOptions: &ExecOptions{
			PodUid:    podUID,
			Container: container,
			Command:   command,
			Stdin:     stdin,
			Stderr:    stderr,
			Stdout:    stdout,
			Tty:       tty,
		},
	})
}

func NewContainerAttachCmd(podUID, container string, stdin, stdout, stderr, tty bool) *Cmd {
	return newPodCmd(0, Attach, &PodCmd_ExecOptions{
		ExecOptions: &ExecOptions{
			PodUid:    podUID,
			Container: container,
			Stdin:     stdin,
			Stderr:    stderr,
			Stdout:    stdout,
			Tty:       tty,
		},
	})
}

func NewContainerLogCmd(podUID, container string, follow, timestamp bool, since time.Time, tailLines int64) *Cmd {
	return newPodCmd(0, Log, &PodCmd_LogOptions{
		LogOptions: &LogOptions{
			PodUid:    podUID,
			Container: container,
			Follow:    follow,
			Timestamp: timestamp,
			Since:     since.UnixNano(),
			TailLines: tailLines,
		},
	})
}

func NewPortForwardCmd(podUID string, port int32, protocol string) *Cmd {
	return newPodCmd(0, PortForward, &PodCmd_PortForwardOptions{
		PortForwardOptions: &PortForwardOptions{
			PodUid:   podUID,
			Port:     port,
			Protocol: protocol,
		},
	})
}

func NewContainerInputCmd(sid uint64, data []byte) *Cmd {
	return newPodCmd(sid, Input, &PodCmd_InputOptions{
		InputOptions: &InputOptions{
			Data: data,
		},
	})
}

func NewContainerTtyResizeCmd(sid uint64, cols uint16, rows uint16) *Cmd {
	return newPodCmd(sid, ResizeTty, &PodCmd_ResizeOptions{
		ResizeOptions: &TtyResizeOptions{
			Cols: uint32(cols),
			Rows: uint32(rows),
		},
	})
}

func NewSessionCloseCmd(sessionToClose uint64) *Cmd {
	return &Cmd{
		Cmd: &Cmd_CloseSession{
			CloseSession: sessionToClose,
		},
	}
}
