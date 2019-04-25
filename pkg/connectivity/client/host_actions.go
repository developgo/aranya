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
	"log"
	"os/exec"

	"github.com/kr/pty"

	"arhat.dev/aranya/pkg/connectivity"
)

var (
	ErrCommandNotProvided = connectivity.NewCommonError("command not provided for exec")
)

func execInHost(stdin io.Reader, stdout, stderr io.Writer, resizeCh <-chan *connectivity.TtyResizeOptions, command []string, tty bool) *connectivity.Error {
	if len(command) == 0 {
		// impossible for agent exec, but still check
		return ErrCommandNotProvided
	}

	cmd := exec.Command(command[0], command[1:]...)

	if tty {
		f, err := pty.Start(cmd)
		if err != nil {
			return connectivity.NewCommonError(err.Error())
		}
		defer func() { _ = f.Close() }()

		go func() {
			if stdin != nil {
				log.Printf("starting to handle input")
				defer log.Printf("finished handling input")

				_, err := io.Copy(f, stdin)
				if err != nil {
					log.Printf("exception heppened when writing: %v", err)
				}
			}
		}()

		go func() {
			if stdout != nil {
				log.Printf("starting to handle output")
				defer log.Printf("finished handling output")

				_, err := io.Copy(stdout, f)
				if err != nil {
					log.Printf("exception happened when reading")
				}
			}
		}()

		go func() {
			for size := range resizeCh {
				err := pty.Setsize(f, &pty.Winsize{Cols: uint16(size.GetCols()), Rows: uint16(size.GetRows())})
				if err != nil {
					log.Printf("failed to resize")
				}
			}
		}()
	} else {
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		cmd.Stdin = stdin

		if err := cmd.Start(); err != nil {
			return connectivity.NewCommonError(err.Error())
		}
	}

	if err := cmd.Wait(); err != nil {
		return connectivity.NewCommonError(err.Error())
	}

	return nil
}
