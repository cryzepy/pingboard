package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

var cgnat = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

func isBlockedIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsUnspecified() ||
		ip.IsMulticast() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		cgnat.Contains(ip)
}

// NewHTTPChecker membuat CheckFunc yang melakukan request GET sungguhan.
//
// Karena server melakukan request ke URL pilihan pengguna, alamat privat
// (localhost, 10.x, 192.168.x, 169.254.x, dst) diblokir secara default untuk
// mencegah SSRF. Pemeriksaan dilakukan pada saat koneksi dibuat, sehingga
// berlaku juga untuk setiap redirect dan hasil resolusi DNS.
func NewHTTPChecker(allowPrivate bool) CheckFunc {
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
		Control: func(network, address string, _ syscall.RawConn) error {
			if allowPrivate {
				return nil
			}
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || isBlockedIP(ip) {
				return errors.New("alamat privat/lokal tidak diizinkan")
			}
			return nil
		},
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 8 * time.Second,
			DisableKeepAlives:     true,
		},
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("terlalu banyak redirect")
			}
			return nil
		},
	}

	return func(ctx context.Context, rawURL string) Check {
		start := time.Now()
		c := Check{Time: start.UTC()}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			c.Error = err.Error()
			return c
		}
		req.Header.Set("User-Agent", "PingBoard/1.0 (+uptime monitor)")

		resp, err := client.Do(req)
		c.LatencyMs = time.Since(start).Milliseconds()
		if err != nil {
			c.Error = shortError(err)
			return c
		}
		defer resp.Body.Close()

		c.StatusCode = resp.StatusCode
		c.Up = resp.StatusCode < 400
		if !c.Up {
			c.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return c
	}
}

func shortError(err error) string {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return "timeout"
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return "DNS gagal: " + dnsErr.Name
	}
	msg := err.Error()
	if len(msg) > 160 {
		msg = msg[:160] + "…"
	}
	return msg
}
