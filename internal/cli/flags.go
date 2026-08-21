package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"ya-music/ya"
)

const defaultOutputDir = "./downloads"

type sharedFlags struct {
	timeoutSeconds int
	skipCover      bool
}

type parseOutcome[T any] struct {
	options  T
	exitCode int
	proceed  bool
}

type tuiOptions struct {
	sharedFlags
}

type downloadOptions struct {
	token  string
	link   string
	format ya.AudioFormat
	output string
	sharedFlags
}

func validateSharedFlags(options sharedFlags) error {
	if options.timeoutSeconds < 0 {
		return errors.New("timeout must be >= 0 seconds")
	}
	return nil
}

func registerSharedFlags(fs *flag.FlagSet, dest *sharedFlags) {
	fs.IntVar(&dest.timeoutSeconds, "timeout", 0, "download timeout in seconds (0 disables timeout)")
	fs.BoolVar(&dest.skipCover, "skip-cover", false, "skip downloading and embedding track cover images")
}

func parseTUIOptions(args []string, stderr io.Writer) parseOutcome[tuiOptions] {
	options := tuiOptions{}
	flags := flag.NewFlagSet("yamdl", flag.ContinueOnError)
	flags.SetOutput(stderr)
	registerSharedFlags(flags, &options.sharedFlags)
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return parseOutcome[tuiOptions]{exitCode: 0}
		}
		return parseOutcome[tuiOptions]{exitCode: 2}
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "unexpected command; use 'yamdl download --help' for batch downloads")
		return parseOutcome[tuiOptions]{exitCode: 2}
	}
	if err := validateSharedFlags(options.sharedFlags); err != nil {
		fmt.Fprintln(stderr, err)
		return parseOutcome[tuiOptions]{exitCode: 2}
	}
	return parseOutcome[tuiOptions]{options: options, proceed: true}
}

func parseDownloadOptions(args []string, stderr io.Writer) parseOutcome[downloadOptions] {
	options := downloadOptions{format: ya.AudioFormatMP3, output: defaultOutputDir}
	flags := flag.NewFlagSet("yamdl download", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&options.token, "token", "", "Yandex Music authentication token (required)")
	flags.StringVar(&options.link, "link", "", "Yandex Music track, album, playlist, or chart URL (required)")
	format := string(options.format)
	flags.StringVar(&format, "format", format, "audio format: mp3 or flac")
	flags.StringVar(&options.output, "output", options.output, "directory for downloaded tracks")
	registerSharedFlags(flags, &options.sharedFlags)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: yamdl download --token TOKEN --link URL [options]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return parseOutcome[downloadOptions]{exitCode: 0}
		}
		return parseOutcome[downloadOptions]{exitCode: 2}
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "download does not accept positional arguments")
		return parseOutcome[downloadOptions]{exitCode: 2}
	}
	if strings.TrimSpace(options.token) == "" {
		fmt.Fprintln(stderr, "--token is required")
		return parseOutcome[downloadOptions]{exitCode: 2}
	}
	if strings.TrimSpace(options.link) == "" {
		fmt.Fprintln(stderr, "--link is required")
		return parseOutcome[downloadOptions]{exitCode: 2}
	}
	if err := validateSharedFlags(options.sharedFlags); err != nil {
		fmt.Fprintln(stderr, err)
		return parseOutcome[downloadOptions]{exitCode: 2}
	}
	options.format = ya.AudioFormat(strings.ToLower(strings.TrimSpace(format)))
	if options.format != ya.AudioFormatMP3 && options.format != ya.AudioFormatFLAC {
		fmt.Fprintln(stderr, "--format must be mp3 or flac")
		return parseOutcome[downloadOptions]{exitCode: 2}
	}
	options.output = strings.TrimSpace(options.output)
	if options.output == "" {
		fmt.Fprintln(stderr, "--output must not be empty")
		return parseOutcome[downloadOptions]{exitCode: 2}
	}
	options.token = strings.TrimSpace(options.token)
	options.link = strings.TrimSpace(options.link)
	return parseOutcome[downloadOptions]{options: options, proceed: true}
}
