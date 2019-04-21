package util

// ScanAnyAvail a split func to get all available bytes
func ScanAnyAvail(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	return len(data), data[:], nil
}
