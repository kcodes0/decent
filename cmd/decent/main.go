package main

import (
	"fmt"
	"os"

	"github.com/kcodes0/decent/internal/cli"
	"github.com/kcodes0/decent/internal/version"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "--version", "-v":
			fmt.Println(version.Current)
			return
		}
	}
	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
