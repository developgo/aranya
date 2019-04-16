package pod

import (
	"bufio"
	"context"
	"fmt"
	"io"

	"k8s.io/client-go/tools/remotecommand"

	"arhat.dev/aranya/pkg/node/connectivity"
	"arhat.dev/aranya/pkg/node/util"
)

func (m *Manager) handleBidirectionalStream(initialCmd *connectivity.Cmd, in io.Reader, out, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize) (err error) {
	if out == nil {
		return fmt.Errorf("output should not be nil")
	}
	defer log.Error(err, "finished stream handle")

	ctx, cancel := context.WithCancel(m.ctx)
	defer cancel()

	msgCh, err := m.remoteManager.PostCmd(ctx, initialCmd)
	if err != nil {
		log.Error(err, "failed to post initial command")
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
			// defer close(inputCh)

			for s.Scan() {
				inputCh <- connectivity.NewContainerInputCmd(sid, s.Bytes())
			}
			log.Error(s.Err(), "finished stream input", "remains", s.Text())
		}()
	}

	defer func() {
		// close out and stderr with best effort
		log.Info("close out writer")
		_ = out.Close()

		if stderr != nil {
			log.Info("close err writer")
			_ = stderr.Close()
		}
	}()

	for {
		select {
		case userInput, more := <-inputCh:
			if !more {
				log.Info("input ch closed")
				return nil
			}
			log.Info("send data", "data", string(userInput.GetPodCmd().GetInputOptions().GetData()))

			_, err = m.remoteManager.PostCmd(ctx, userInput)
			if err != nil {
				log.Error(err, "failed to post user input")
				return err
			}
		case msg, more := <-msgCh:
			if !more {
				log.Info("msg ch closed")
				return nil
			}
			log.Info("recv data", "data", string(msg.GetData().GetData()))
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
					log.Error(err, "failed to write output")
					return err
				}
			}
		case size, more := <-resizeCh:
			if !more {
				log.Info("resize ch closed")
				return nil
			}
			log.Info("resize")

			resizeCmd := connectivity.NewContainerTtyResizeCmd(sid, size.Width, size.Height)
			_, err = m.remoteManager.PostCmd(ctx, resizeCmd)
			if err != nil {
				return err
			}
		}
	}
}
