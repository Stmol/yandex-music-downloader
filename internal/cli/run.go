package cli

import "io"

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "download" {
		return runDownload(args[1:], stdout, stderr)
	}
	return runTUI(args, stderr)
}
