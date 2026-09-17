package core

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

const (
	MimeTypeJSON           = "application/json"
	MimeTypeHTML           = "text/html"
	MimeTypeJavaScript     = "application/javascript"
	MimeTypeJavaScriptText = "text/javascript"
)

// ClientIP returns the IP address of the visitor: the device that sent the
// original request.
//
// A request can reach the server in two ways. It can come straight from the
// visitor, or it can pass through a proxy: a server in front of this one that
// forwards requests to it. The address of the connection itself is
// r.RemoteAddr, and with a proxy in the path that address belongs to the
// proxy. The visitor's address then travels in a header, named by a config
// setting. This function returns the address from that header when the header
// is configured and present. Otherwise it returns the connection address.
//
// The result has the same text form for the same address, however the address
// was written in the request. See normalizeIP.
func (a *App) ClientIP(r *http.Request) string {
	ip := normalizeIP(r.RemoteAddr)

	header := a.Config().Server.ClientIpProxyHeader
	if header == "" {
		return ip
	}

	forwarded := r.Header.Get(header)
	if forwarded == "" {
		return ip
	}

	// A forwarded header holds one address per proxy in the path, separated
	// by commas. By convention the first entry is the visitor's, because each
	// proxy adds the address it received the request from.
	parts := strings.Split(forwarded, ",")
	forwardedIP := strings.TrimSpace(parts[0])
	return normalizeIP(forwardedIP)
}

// normalizeIP returns one fixed text form for an IP address.
//
// It accepts the shapes that appear in requests: "192.0.2.1",
// "192.0.2.1:443", "[2001:db8::1]:443" and "2001:db8::1". An IPv4 address
// written in its IPv6 form ("::ffff:192.0.2.1") becomes "192.0.2.1", and an
// IPv6 address is shortened to its compressed form ("2001:db8::1"). A value
// that is not an address is returned unchanged.
func normalizeIP(raw string) string {
	host, _, err := net.SplitHostPort(raw)
	if err != nil {
		host = raw
	}

	addr, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}

	return addr.Unmap().String()
}
