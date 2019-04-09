package main

import (
	"fmt"
	"os"

	arhatInternal "arhat.dev/aranya/cmd/arhat/internal"
)

func main() {
	cmd := arhatInternal.NewArhatCmd()

	if err := cmd.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "execute cmd failed")
		os.Exit(1)
	}
}
