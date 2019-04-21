package main

import (
	"fmt"
	"os"

	aranyaInternal "arhat.dev/aranya/cmd/aranya/internal"
)

func main() {
	cmd := aranyaInternal.NewAranyaCmd()

	if err := cmd.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "execute cmd failed")
		os.Exit(1)
	}
}
