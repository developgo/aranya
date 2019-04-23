package connectivity

func (m *Msg) Err() *Error {
	switch m.GetError().GetKind() {
	case ErrCommon:
		if m.GetError().GetDescription() != "" {
			return m.GetError()
		}

		return nil
	default:
		return m.GetError()
	}
}

func (m *Error) Error() string {
	return m.GetDescription()
}

func (m *Error) IsCommon() bool {
	return m.isKind(ErrCommon)
}

func (m *Error) IsNotFound() bool {
	return m.isKind(ErrNotFound)
}

func (m *Error) IsAlreadyExists() bool {
	return m.isKind(ErrAlreadyExists)
}

func (m *Error) IsNotSupported() bool {
	return m.isKind(ErrNotSupported)
}

func (m *Error) isKind(kind Error_Kind) bool {
	if m == nil {
		return false
	}

	return m.Kind == kind
}
