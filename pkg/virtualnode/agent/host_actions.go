package agent

import (
	"io"
	"log"
	"os/exec"

	"github.com/kr/pty"
	"k8s.io/client-go/tools/remotecommand"

	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

var (
	ErrCommandNotProvided = connectivity.NewCommonError("command not provided for exec")
)

func execInHost(stdin io.Reader, stdout, stderr io.Writer, resizeCh <-chan remotecommand.TerminalSize, command []string, tty bool) *connectivity.Error {
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
				err := pty.Setsize(f, &pty.Winsize{Cols: size.Width, Rows: size.Height})
				if err != nil {
					log.Printf("failed to resize: cols = %d, rows = %d", size.Width, size.Height)
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
