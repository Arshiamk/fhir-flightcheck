package controlplane

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var sensitiveValue = regexp.MustCompile(`(?i)(authorization|token|secret|password|api[-_]?key)(["'=:\s]+)([^&\s",}]+)`)

func Redact(value string) string {
	return sensitiveValue.ReplaceAllString(value, `$1$2[REDACTED]`)
}

type URLPolicy struct {
	Resolver       *net.Resolver
	AllowLocalDemo bool
	ResolveTimeout time.Duration
}

func (p URLPolicy) Validate(ctx context.Context, raw string, targetAllowsPrivate bool) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" {
		return nil, errors.New("baseUrl must be an absolute URL")
	}
	if parsed.User != nil {
		return nil, errors.New("baseUrl must not contain credentials")
	}
	allowPrivate := p.AllowLocalDemo && targetAllowsPrivate
	if parsed.Scheme != "https" && !(allowPrivate && parsed.Scheme == "http") {
		return nil, errors.New("baseUrl must use HTTPS; HTTP is limited to explicit local demo targets")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("baseUrl must not contain a query or fragment")
	}
	if err := p.validateHost(ctx, parsed.Hostname(), allowPrivate); err != nil {
		return nil, err
	}
	return parsed, nil
}

func (p URLPolicy) validateHost(ctx context.Context, host string, allowPrivate bool) error {
	_, err := p.resolveAndValidate(ctx, host, allowPrivate)
	return err
}

func (p URLPolicy) resolveAndValidate(ctx context.Context, host string, allowPrivate bool) ([]net.IPAddr, error) {
	if strings.EqualFold(host, "localhost") && !allowPrivate {
		return nil, errors.New("private and local target addresses are blocked")
	}
	timeout := p.ResolveTimeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	resolveCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resolver := p.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	addresses, err := resolver.LookupIPAddr(resolveCtx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve target host: %w", err)
	}
	if len(addresses) == 0 {
		return nil, errors.New("target host did not resolve")
	}
	for _, address := range addresses {
		if err := validateIP(address.IP, allowPrivate); err != nil {
			return nil, err
		}
	}
	return addresses, nil
}

func validateIP(ip net.IP, allowPrivate bool) error {
	if ip == nil || ip.IsUnspecified() || ip.IsMulticast() {
		return errors.New("unspecified or multicast target addresses are blocked")
	}
	private := ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
	if private && !allowPrivate {
		return errors.New("private and local target addresses are blocked")
	}
	return nil
}

func (p URLPolicy) HTTPClient(targetAllowsPrivate bool, timeout time.Duration) *http.Client {
	allowPrivate := p.AllowLocalDemo && targetAllowsPrivate
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          20,
		MaxIdleConnsPerHost:   4,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: timeout,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			addresses, err := p.resolveAndValidate(ctx, host, allowPrivate)
			if err != nil {
				return nil, err
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(addresses[0].IP.String(), port))
		},
	}
	client := &http.Client{Transport: transport, Timeout: timeout}
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return errors.New("redirect limit exceeded")
		}
		if _, err := p.Validate(request.Context(), request.URL.String(), targetAllowsPrivate); err != nil {
			return fmt.Errorf("unsafe redirect: %w", err)
		}
		return nil
	}
	return client
}
