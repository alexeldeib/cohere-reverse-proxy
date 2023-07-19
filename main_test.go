package main_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/alexeldeib/cohere-reverse-proxy/internal"
	"github.com/stretchr/testify/assert"
)

func Test_Proxy_Origin_Request(t *testing.T) {
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "reverse proxied")
	}))
	defer backendServer.Close()

	targetUrl, err := url.Parse(backendServer.URL)
	if err != nil {
		t.Fatal(err)
	}

	proxy := internal.NewProxy(targetUrl)

	frontendServer := httptest.NewServer(proxy)
	defer frontendServer.Close()

	resp, err := http.Get(frontendServer.URL)
	if err != nil {
		t.Fatal(err)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, string(b), "reverse proxied\n")
}

func Test_Proxy_XForwarded(t *testing.T) {
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		xForwardedFor := r.Header.Get("X-Forwarded-For")
		xForwardedHost := r.Header.Get("X-Forwarded-Host")
		fmt.Fprintf(w, "X-Forwarded-For: %s\n", xForwardedFor)
		fmt.Fprintf(w, "X-Forwarded-Host: %s\n", xForwardedHost)
	}))
	defer backendServer.Close()

	targetUrl, err := url.Parse(backendServer.URL)
	if err != nil {
		t.Fatal(err)
	}

	proxy := internal.NewProxy(targetUrl)

	frontendServer := httptest.NewServer(proxy)
	defer frontendServer.Close()

	resp, err := http.Get(frontendServer.URL)
	if err != nil {
		t.Fatal(err)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	assert.Contains(t, string(b), "X-Forwarded-For: 127.0.0.1\n")
	assert.Contains(t, string(b), "X-Forwarded-Host: 127.0.0.1:")
}

func Test_Proxy_Trailers(t *testing.T) {
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Trailer", "X-Trailer,X-Trailer-2")
		w.WriteHeader(http.StatusOK)
		xForwardedFor := r.Header.Get("X-Forwarded-For")
		fmt.Fprintf(w, "X-Forwarded-For: %s\n", xForwardedFor)
		w.Header().Set("X-Trailer", "first trailer")
		w.Header().Set("X-Trailer-2", "second trailer")
	}))
	defer backendServer.Close()

	targetUrl, err := url.Parse(backendServer.URL)
	if err != nil {
		t.Fatal(err)
	}

	proxy := internal.NewProxy(targetUrl)

	frontendServer := httptest.NewServer(proxy)
	defer frontendServer.Close()

	resp, err := http.Get(frontendServer.URL)
	if err != nil {
		t.Fatal(err)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, string(b), "X-Forwarded-For: 127.0.0.1\n")
	assert.Equal(t, len(resp.Trailer), 2)
	assert.Equal(t, resp.Trailer.Get("X-Trailer"), "first trailer")
	assert.Equal(t, resp.Trailer.Get("X-Trailer-2"), "second trailer")
}

func Test_Live_Server_Request(t *testing.T) {
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		xForwardedFor := r.Header.Get("X-Forwarded-For")
		assert.NotEmpty(t, xForwardedFor)
		fmt.Fprintln(w, xForwardedFor)
	}))
	defer backendServer.Close()

	targetUrl, err := url.Parse(backendServer.URL)
	if err != nil {
		t.Fatal(err)
	}

	srv := internal.NewServer(targetUrl)

	assert.NoError(t, srv.Listen("127.0.0.1:0"))

	go srv.Serve()
	defer srv.Shutdown(context.Background())

	resp, err := http.Get(srv.URL())
	if err != nil {
		t.Fatal(err)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, string(b), "127.0.0.1\n")
}

func Test_Live_Server_Fails_Calling_Serve_Without_Listen(t *testing.T) {
	srv := internal.NewServer(&url.URL{})
	err := srv.Serve()
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "must call Listen() before Serve()")
}
