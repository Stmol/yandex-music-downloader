package cli

import (
	"fmt"
	"io"
	"ya-music/internal/version"
)

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "--version":
			if len(args) != 1 {
				fmt.Fprintln(stderr, "--version does not accept arguments")
				return 2
			}
			fmt.Fprintln(stdout, version.Version)
			return 0
		case "download":
			return runDownload(args[1:], stdout, stderr)
		}
	}
	return runTUI(args, stderr)
}
