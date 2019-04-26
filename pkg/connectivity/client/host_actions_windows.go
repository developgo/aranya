// +build windows

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

package client

import (
	"io"

	"arhat.dev/aranya/pkg/connectivity"
)

func execInHost(stdin io.Reader, stdout, stderr io.Writer, resizeCh <-chan *connectivity.TtyResizeOptions, command []string, tty bool) *connectivity.Error {
	// TODO: ignore tty and implement exec
	return connectivity.NewNotSupportedError("exec to host not supported on windows")
}
