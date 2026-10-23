package utils

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func mustParseProxyURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := ParseProxyURL(raw)
	if err != nil {
		t.Fatalf("ParseProxyURL(%q): %v", raw, err)
	}
	return u
}

func TestParseProxyURLAcceptsValidURLs(t *testing.T) {
	tests := []struct {
		raw      string
		wantHost string
	}{
		{"http://127.0.0.1:8080", "127.0.0.1:8080"},
		{"http://127.0.0.1", "127.0.0.1:80"},       // default port
		{"https://proxy.local", "proxy.local:443"}, // default port
		{"socks5://127.0.0.1:1080", "127.0.0.1:1080"},
		{"socks5://127.0.0.1", "127.0.0.1:1080"}, // default port
		{"  socks5://127.0.0.1:1080  ", "127.0.0.1:1080"},
		{"http://user:secret@proxy.local:8080", "proxy.local:8080"},
		{"http://proxy.local/", "proxy.local:80"},
	}

	for _, test := range tests {
		u, err := ParseProxyURL(test.raw)
		if err != nil {
			t.Fatalf("ParseProxyURL(%q): unexpected error: %v", test.raw, err)
		}
		if u.Host != test.wantHost {
			t.Errorf("ParseProxyURL(%q).Host = %q, want %q", test.raw, u.Host, test.wantHost)
		}
	}
}

func TestParseProxyURLRejectsInvalidURLs(t *testing.T) {
	tests := []string{
		"",
		"   ",
		"ftp://127.0.0.1:21",       // unsupported scheme
		"socks4://127.0.0.1:1080",  // unsupported scheme
		"http://",                  // no host
		"http://:8080",             // no host
		"127.0.0.1:8080",           // no scheme
		"http://proxy.local/a/b",   // path is ignored otherwise
		"http://proxy.local/?q=1",  // query is ignored otherwise
		"http://user@proxy.local",  // empty password
		"http://:pass@proxy.local", // empty username
		"http://user:sec ret@host", // invalid character in userinfo
	}

	for _, raw := range tests {
		if u, err := ParseProxyURL(raw); err == nil {
			t.Errorf("ParseProxyURL(%q) = %v, want error", raw, u)
		}
	}
}

func TestParseProxyURLErrorsNeverLeakCredentials(t *testing.T) {
	// Every failing input below carries user:secret; none of it may appear
	// in the error text.
	tests := []string{
		"ftp://user:secret@proxy.local:21",
		"http://user:secret@",
		"http://user:secret@proxy.local/path",
		"http://user:secret@proxy.local?q=1",
		"http://user:sec ret@proxy.local",
		"socks5://user@proxy.local",
	}

	for _, raw := range tests {
		_, err := ParseProxyURL(raw)
		if err == nil {
			t.Fatalf("ParseProxyURL(%q): want error", raw)
		}
		if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "user:sec") {
			t.Errorf("ParseProxyURL(%q) error leaks credentials: %q", raw, err)
		}
	}
}

func TestHTTPProxyTransportRoutesThroughProxy(t *testing.T) {
	proxyHits := 0
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHits++
		if !r.URL.IsAbs() {
			t.Errorf("proxy received origin-form request for %q", r.URL)
		}
		_, _ = w.Write([]byte("served by proxy"))
	}))
	defer proxyServer.Close()

	originHits := 0
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originHits++
		_, _ = w.Write([]byte("served by origin"))
	}))
	defer origin.Close()

	client := NewHttpClient()
	if err := client.SetProxy(mustParseProxyURL(t, proxyServer.URL)); err != nil {
		t.Fatalf("SetProxy: %v", err)
	}

	body, err := client.Get(origin.URL)
	if err != nil {
		t.Fatalf("Get via proxy: %v", err)
	}
	if string(body) != "served by proxy" {
		t.Fatalf("body = %q, want proxy response", body)
	}
	if proxyHits != 1 {
		t.Fatalf("proxyHits = %d, want 1", proxyHits)
	}
	if originHits != 0 {
		t.Fatalf("originHits = %d, want 0: request must not go direct", originHits)
	}
}

func TestSOCKS5TransportWiring(t *testing.T) {
	rt, err := NewProxyTransport(mustParseProxyURL(t, "socks5://127.0.0.1:1080"))
	if err != nil {
		t.Fatalf("NewProxyTransport: %v", err)
	}
	transport, ok := rt.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", rt)
	}
	if transport.DialContext == nil {
		t.Fatal("SOCKS5 transport must dial through the proxy")
	}
	if transport.Proxy != nil {
		t.Fatal("SOCKS5 transport must not use HTTP proxying")
	}
}

// socks5TestServer is a minimal SOCKS5 server: it supports CONNECT with
// no-auth or username/password negotiation and pipes bytes to the target.
type socks5TestServer struct {
	listener    net.Listener
	requireAuth bool
	username    string
	password    string
	connects    chan string
}

func newSocks5TestServer(t *testing.T) *socks5TestServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := &socks5TestServer{listener: listener, connects: make(chan string, 8)}
	go server.serve()
	t.Cleanup(func() { _ = listener.Close() })
	return server
}

func (s *socks5TestServer) addr() string {
	return s.listener.Addr().String()
}

func (s *socks5TestServer) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *socks5TestServer) handle(conn net.Conn) {
	defer conn.Close()

	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil || header[0] != 0x05 {
		return
	}
	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(conn, methods); err != nil {
		return
	}

	if s.requireAuth {
		if _, err := conn.Write([]byte{0x05, 0x02}); err != nil {
			return
		}
		authHeader := make([]byte, 2)
		if _, err := io.ReadFull(conn, authHeader); err != nil || authHeader[0] != 0x01 {
			return
		}
		username := make([]byte, int(authHeader[1]))
		if _, err := io.ReadFull(conn, username); err != nil {
			return
		}
		if _, err := io.ReadFull(conn, authHeader[:1]); err != nil {
			return
		}
		password := make([]byte, int(authHeader[0]))
		if _, err := io.ReadFull(conn, password); err != nil {
			return
		}
		status := byte(0x00)
		if string(username) != s.username || string(password) != s.password {
			status = 0x01
		}
		if _, err := conn.Write([]byte{0x01, status}); err != nil || status != 0x00 {
			return
		}
	} else if _, err := conn.Write([]byte{0x05, 0x00}); err != nil {
		return
	}

	request := make([]byte, 4)
	if _, err := io.ReadFull(conn, request); err != nil || request[1] != 0x01 {
		return
	}

	var host string
	switch request[3] {
	case 0x01:
		raw := make([]byte, 4)
		if _, err := io.ReadFull(conn, raw); err != nil {
			return
		}
		host = net.IP(raw).String()
	case 0x03:
		if _, err := io.ReadFull(conn, request[:1]); err != nil {
			return
		}
		raw := make([]byte, int(request[0]))
		if _, err := io.ReadFull(conn, raw); err != nil {
			return
		}
		host = string(raw)
	default:
		return
	}

	portRaw := make([]byte, 2)
	if _, err := io.ReadFull(conn, portRaw); err != nil {
		return
	}
	target := net.JoinHostPort(host, fmt.Sprint(binary.BigEndian.Uint16(portRaw)))

	upstream, err := net.DialTimeout("tcp", target, 5*time.Second)
	if err != nil {
		_, _ = conn.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer upstream.Close()

	if _, err := conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}
	s.connects <- target

	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(upstream, conn)
		close(done)
	}()
	_, _ = io.Copy(conn, upstream)
	<-done
}

func TestSOCKS5ProxyEndToEnd(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("served via socks5"))
	}))
	defer origin.Close()

	proxyServer := newSocks5TestServer(t)

	client := NewHttpClient()
	if err := client.SetProxy(mustParseProxyURL(t, "socks5://"+proxyServer.addr())); err != nil {
		t.Fatalf("SetProxy: %v", err)
	}

	body, err := client.Get(origin.URL)
	if err != nil {
		t.Fatalf("Get via SOCKS5: %v", err)
	}
	if string(body) != "served via socks5" {
		t.Fatalf("body = %q", body)
	}

	select {
	case target := <-proxyServer.connects:
		want := strings.TrimPrefix(origin.URL, "http://")
		if target != want {
			t.Fatalf("proxy CONNECT target = %q, want %q", target, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("SOCKS5 proxy never received a CONNECT request")
	}
}

func TestSOCKS5ProxyEndToEndWithAuth(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("served via authenticated socks5"))
	}))
	defer origin.Close()

	proxyServer := newSocks5TestServer(t)
	proxyServer.requireAuth = true
	proxyServer.username = "user"
	proxyServer.password = "secret"

	client := NewHttpClient()
	proxyURL := "socks5://user:secret@" + proxyServer.addr()
	if err := client.SetProxy(mustParseProxyURL(t, proxyURL)); err != nil {
		t.Fatalf("SetProxy: %v", err)
	}

	body, err := client.Get(origin.URL)
	if err != nil {
		t.Fatalf("Get via authenticated SOCKS5: %v", err)
	}
	if string(body) != "served via authenticated socks5" {
		t.Fatalf("body = %q", body)
	}
}
