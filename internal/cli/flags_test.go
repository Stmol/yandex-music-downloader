package cli

import (
	"bytes"
	"strings"
	"testing"
	"ya-music/ya"
)

func TestParseTUIOptionsRejectsPositionalArgs(t *testing.T) {
	var stderr bytes.Buffer
	parsed := parseTUIOptions([]string{"download"}, &stderr)
	if parsed.proceed || parsed.exitCode != 2 {
		t.Fatalf("proceed = %v, exit code = %d, want 2, stderr: %s", parsed.proceed, parsed.exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "unexpected command; use 'yamdl download --help' for batch downloads") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestParseTUIOptionsAcceptsTimeoutAndSkipCover(t *testing.T) {
	var stderr bytes.Buffer
	parsed := parseTUIOptions([]string{"--timeout", "30", "--skip-cover"}, &stderr)
	if !parsed.proceed {
		t.Fatalf("expected proceed, exit code = %d, stderr: %s", parsed.exitCode, stderr.String())
	}
	if parsed.options.timeoutSeconds != 30 || !parsed.options.skipCover {
		t.Fatalf("unexpected options: %#v", parsed.options)
	}
}

func TestParseTUIOptionsRejectsNegativeTimeout(t *testing.T) {
	var stderr bytes.Buffer
	parsed := parseTUIOptions([]string{"--timeout", "-1"}, &stderr)
	if parsed.proceed || parsed.exitCode != 2 {
		t.Fatalf("proceed = %v, exit code = %d, stderr: %s", parsed.proceed, parsed.exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "timeout must be >= 0 seconds") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestParseDownloadOptionsRejectsNegativeTimeout(t *testing.T) {
	var stderr bytes.Buffer
	parsed := parseDownloadOptions([]string{
		"--token", "abc123",
		"--link", "https://music.yandex.ru/album/123",
		"--timeout", "-1",
	}, &stderr)
	if parsed.proceed || parsed.exitCode != 2 {
		t.Fatalf("proceed = %v, exit code = %d, stderr: %s", parsed.proceed, parsed.exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "timeout must be >= 0 seconds") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestParseDownloadOptionsAcceptsCurrentDownloadFlags(t *testing.T) {
	var stderr bytes.Buffer
	parsed := parseDownloadOptions([]string{
		"--token", "  abc123  ",
		"--link", "  https://music.yandex.ru/album/123  ",
		"--format", "FLAC",
		"--output", "./music",
		"--timeout", "180",
		"--proxy", "socks5://127.0.0.1:1080",
		"--skip-cover",
	}, &stderr)

	if !parsed.proceed {
		t.Fatalf("expected proceed, exit code = %d, stderr: %s", parsed.exitCode, stderr.String())
	}
	options := parsed.options
	if options.token != "abc123" || options.link != "https://music.yandex.ru/album/123" {
		t.Fatalf("unexpected required options: %#v", options)
	}
	if options.format != ya.AudioFormatFLAC || options.output != "./music" || options.timeoutSeconds != 180 || !options.skipCover {
		t.Fatalf("unexpected parsed options: %#v", options)
	}
	if options.proxy != "socks5://127.0.0.1:1080" {
		t.Fatalf("proxy = %q", options.proxy)
	}
	if options.proxyURL == nil || options.proxyURL.Host != "127.0.0.1:1080" {
		t.Fatalf("proxyURL = %v", options.proxyURL)
	}
}

func TestParseDownloadOptionsTrimsProxyValue(t *testing.T) {
	var stderr bytes.Buffer
	parsed := parseDownloadOptions([]string{
		"--token", "abc123",
		"--link", "https://music.yandex.ru/album/123",
		"--proxy", "  http://127.0.0.1  ",
	}, &stderr)

	if !parsed.proceed {
		t.Fatalf("expected proceed, exit code = %d, stderr: %s", parsed.exitCode, stderr.String())
	}
	if parsed.options.proxy != "http://127.0.0.1" {
		t.Fatalf("proxy = %q, want trimmed value", parsed.options.proxy)
	}
	if parsed.options.proxyURL == nil || parsed.options.proxyURL.Host != "127.0.0.1:80" {
		t.Fatalf("proxyURL = %v, want default port filled in", parsed.options.proxyURL)
	}
}

func TestParseDownloadOptionsRejectsInvalidProxy(t *testing.T) {
	tests := [][]string{
		{"--proxy", "ftp://127.0.0.1:21"},
		{"--proxy", "http://"},
		{"--proxy", "http://proxy.local/path"},
		{"--proxy", "http://user:sec ret@proxy.local"},
	}

	for _, extra := range tests {
		args := append([]string{
			"--token", "abc123",
			"--link", "https://music.yandex.ru/album/123",
		}, extra...)

		var stderr bytes.Buffer
		parsed := parseDownloadOptions(args, &stderr)
		if parsed.proceed || parsed.exitCode != 2 {
			t.Fatalf("args %q: proceed = %v, exit code = %d, want exit 2", args, parsed.proceed, parsed.exitCode)
		}
		if !strings.Contains(stderr.String(), "invalid --proxy value") {
			t.Fatalf("args %q: stderr = %q", args, stderr.String())
		}
		if strings.Contains(stderr.String(), "sec ret") {
			t.Fatalf("args %q: stderr leaks credentials: %q", args, stderr.String())
		}
	}
}

func TestParseTUIOptionsHandlesProxy(t *testing.T) {
	var stderr bytes.Buffer
	parsed := parseTUIOptions([]string{"--proxy", "socks5://127.0.0.1:1080"}, &stderr)
	if !parsed.proceed {
		t.Fatalf("expected proceed, exit code = %d, stderr: %s", parsed.exitCode, stderr.String())
	}
	if parsed.options.proxyURL == nil || parsed.options.proxyURL.Host != "127.0.0.1:1080" {
		t.Fatalf("proxyURL = %v", parsed.options.proxyURL)
	}

	stderr.Reset()
	parsed = parseTUIOptions([]string{"--proxy", "ftp://127.0.0.1"}, &stderr)
	if parsed.proceed || parsed.exitCode != 2 {
		t.Fatalf("invalid proxy: proceed = %v, exit code = %d, want exit 2", parsed.proceed, parsed.exitCode)
	}
}

func TestParseDownloadOptionsRejectsInvalidInput(t *testing.T) {
	tests := [][]string{
		{"--link", "https://music.yandex.ru/album/123"},
		{"--token", "abc123"},
		{"--token", "abc123", "--link", "https://music.yandex.ru/album/123", "--format", "aac"},
		{"--token", "abc123", "--link", "https://music.yandex.ru/album/123", "--timeout", "-1"},
	}

	for _, args := range tests {
		var stderr bytes.Buffer
		parsed := parseDownloadOptions(args, &stderr)
		if parsed.proceed || parsed.exitCode != 2 {
			t.Fatalf("args %q: proceed = %v, exit code = %d, stderr: %s", args, parsed.proceed, parsed.exitCode, stderr.String())
		}
	}
}

func TestParseDownloadOptionsRejectsPositionalArgs(t *testing.T) {
	var stderr bytes.Buffer
	parsed := parseDownloadOptions([]string{
		"--token", "abc",
		"--link", "https://music.yandex.ru/album/1",
		"extra",
	}, &stderr)
	if parsed.proceed || parsed.exitCode != 2 {
		t.Fatalf("proceed = %v, exit code = %d, stderr: %s", parsed.proceed, parsed.exitCode, stderr.String())
	}
}

func TestParseDownloadOptionsUsesEnvVariable(t *testing.T) {
	t.Setenv("YAMDL_DOWNLOAD_DIR", "/env/download/path")

	var stderr bytes.Buffer
	parsed := parseDownloadOptions([]string{
		"--token", "abc123",
		"--link", "https://music.yandex.ru/album/123",
	}, &stderr)

	if !parsed.proceed {
		t.Fatalf("expected proceed, exit code = %d, stderr: %s", parsed.exitCode, stderr.String())
	}
	if parsed.options.output != "/env/download/path" {
		t.Fatalf("expected output = %q, got %q", "/env/download/path", parsed.options.output)
	}
}

func TestParseDownloadOptionsFlagOverridesEnv(t *testing.T) {
	t.Setenv("YAMDL_DOWNLOAD_DIR", "/env/download/path")

	var stderr bytes.Buffer
	parsed := parseDownloadOptions([]string{
		"--token", "abc123",
		"--link", "https://music.yandex.ru/album/123",
		"--output", "/flag/download/path",
	}, &stderr)

	if !parsed.proceed {
		t.Fatalf("expected proceed, exit code = %d, stderr: %s", parsed.exitCode, stderr.String())
	}
	if parsed.options.output != "/flag/download/path" {
		t.Fatalf("expected output = %q, got %q", "/flag/download/path", parsed.options.output)
	}
}

func TestParseDownloadOptionsAcceptsArtistLink(t *testing.T) {
	var stderr bytes.Buffer
	parsed := parseDownloadOptions([]string{
		"--token", "abc123",
		"--link", "https://music.yandex.ru/artist/170660",
	}, &stderr)
	if !parsed.proceed {
		t.Fatalf("expected proceed, exit code = %d, stderr: %s", parsed.exitCode, stderr.String())
	}
	if parsed.options.link != "https://music.yandex.ru/artist/170660" {
		t.Fatalf("link = %q", parsed.options.link)
	}
}
