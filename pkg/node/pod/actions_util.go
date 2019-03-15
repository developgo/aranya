package pod

import (
	"bufio"
	"fmt"
	"io"
	"time"

	"k8s.io/client-go/tools/remotecommand"

	"arhat.dev/aranya/pkg/node/connectivity"
	connectivitySrv "arhat.dev/aranya/pkg/node/connectivity/server"
	"arhat.dev/aranya/pkg/node/util"
)

func (m *Manager) handleBidirectionalStream(initialCmd *connectivity.Cmd, timeout time.Duration, in io.Reader, out, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize) (err error) {
	if out == nil {
		return fmt.Errorf("output should not be nil")
	}

	msgCh, err := m.remoteManager.PostCmd(initialCmd, timeout)
	if err != nil {
		return err
	}

	sid := initialCmd.GetSessionId()

	// generalize resizeCh (or we may need to use reflect, which is inefficient)
	if resizeCh == nil {
		resizeCh = make(chan remotecommand.TerminalSize)
	}

	// read user input if needed
	inputCh := make(chan *connectivity.Cmd, 1)
	if in != nil {
		s := bufio.NewScanner(in)
		s.Split(util.ScanAnyAvail)

		go func() {
			defer close(inputCh)

			for s.Scan() {
				inputCh <- connectivitySrv.NewContainerInputCmd(sid, s.Bytes())
			}
		}()
	}

	defer func() {
		// close out and stderr with best effort
		_ = out.Close()

		if stderr != nil {
			_ = stderr.Close()
		}
	}()

	for {
		select {
		case userInput, more := <-inputCh:
			if !more {
				return nil
			}
			_, err = m.remoteManager.PostCmd(userInput, timeout)
			if err != nil {
				return nil
			}
		case msg, more := <-msgCh:
			if !more {
				return nil
			}
			// only PodData will be received in this session
			switch m := msg.GetMsg().(type) {
			case *connectivity.Msg_Data:
				targetOutput := out
				switch m.Data.GetKind() {
				case connectivity.OTHER, connectivity.STDOUT:
					targetOutput = out
				case connectivity.STDERR:
					if stderr != nil {
						targetOutput = stderr
					}
				default:
					return fmt.Errorf("data kind unknown")
				}

				_, err = targetOutput.Write(m.Data.GetData())
				if err != nil {
					return err
				}
			}
		case size, more := <-resizeCh:
			if !more {
				return nil
			}
			resizeCmd := connectivitySrv.NewContainerTtyResizeCmd(sid, size.Width, size.Height)
			_, err = m.remoteManager.PostCmd(resizeCmd, 0)
			if err != nil {
				return err
			}
		}
	}
}
