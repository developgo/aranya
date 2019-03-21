package main

import (
	"arhat.dev/aranya/cmd/arhat/internal"
	"fmt"
	"os"
)

func main() {
	cmd := internal.NewArhatCmd()
	if err := cmd.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "execute cmd failed")
		os.Exit(1)
	}
}
