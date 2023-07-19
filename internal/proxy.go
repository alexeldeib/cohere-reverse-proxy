package internal

import (
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"golang.org/x/net/http2"
)

// NewProxy configures a reverse proxy handler for a single upstream target.
func NewProxy(target *url.URL) *httputil.ReverseProxy {
	// create our own non-default transport with reasonable timeouts.
	transport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		// Note: this disables H2 in some cases. We're not using it.
		ExpectContinueTimeout: 1 * time.Second,
	}

	// not really used, but would be necessary for HTTP/2
	// if the upstream supports http2 + https, this may matter.
	// for https listening endpoint, would additionally need to:
	// - generate a keypair, add it to http.Server.TLSConfig
	// - change Serve() to ServeTLS()
	// - ensure upstream target for proxy also supports H2
	http2.ConfigureTransport(transport)

	return &httputil.ReverseProxy{
		Transport: transport,
		// Periodically flush data to the client while copying the response body.
		// Ensures correct streaming behavior.
		FlushInterval: 10 * time.Millisecond,
		Rewrite: func(r *httputil.ProxyRequest) {
			// Be a good neighbor and tell upstream who we're forwarding requests for.
			r.SetXForwarded()
			r.SetURL(target)
		},
	}
}
