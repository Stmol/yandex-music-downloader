package utils

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

const ProxyDialTimeout = 30 * time.Second

func ParseProxyURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("proxy URL is empty")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return nil, errors.New("proxy URL is not a valid URL")
	}

	switch u.Scheme {
	case "http", "https", "socks5":
	default:
		return nil, fmt.Errorf("unsupported proxy scheme %q: want http, https, or socks5", u.Scheme)
	}

	if u.Hostname() == "" {
		return nil, errors.New("proxy URL must include a host")
	}
	if u.Path != "" && u.Path != "/" {
		return nil, errors.New("proxy URL must not include a path")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("proxy URL must not include a query or fragment")
	}
	if u.User != nil {
		if u.User.Username() == "" {
			return nil, errors.New("proxy username must not be empty")
		}
		if password, ok := u.User.Password(); !ok || password == "" {
			return nil, errors.New("proxy password must not be empty")
		}
	}

	port := u.Port()
	if port == "" {
		port = defaultProxyPort(u.Scheme)
	}
	u.Host = net.JoinHostPort(u.Hostname(), port)

	return u, nil
}

func defaultProxyPort(scheme string) string {
	switch scheme {
	case "https":
		return "443"
	case "socks5":
		return "1080"
	default:
		return "80"
	}
}

func NewProxyTransport(u *url.URL) (http.RoundTripper, error) {
	dialer := &net.Dialer{
		Timeout:   ProxyDialTimeout,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	if u.Scheme != "socks5" {
		transport.Proxy = http.ProxyURL(u)
		return transport, nil
	}

	var auth *proxy.Auth
	if u.User != nil {
		password, _ := u.User.Password()
		auth = &proxy.Auth{User: u.User.Username(), Password: password}
	}

	socksDialer, err := proxy.SOCKS5("tcp", u.Host, auth, dialer)
	if err != nil {
		return nil, errors.New("failed to configure the SOCKS5 proxy")
	}

	transport.DialContext = socksDialer.(proxy.ContextDialer).DialContext
	return transport, nil
}
