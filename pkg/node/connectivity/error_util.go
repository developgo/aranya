package connectivity

import (
	"errors"
)

func (m *Msg) Error() error {
	switch msg := m.GetMsg().(type) {
	case *Msg_Ack:
		switch msgAck := msg.Ack.GetValue().(type) {
		case *Ack_Error:
			return errors.New(msgAck.Error)
		}
	}

	return nil
}