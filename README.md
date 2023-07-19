# cohere-reverse-proxy

It's an HTTP reverse proxy server!

Credits:
- Traefik Labs' FOSDEM 2019 Talk: https://www.youtube.com/watch?v=tWSmUsYLiE4
- The Go standard library's `httputil.ReverseProxy` and its authors.
- Cloudflare's blog posts on HTTP timeouts and attacks like slow loris
  - https://www.cloudflare.com/learning/ddos/ddos-attack-tools/slowloris/
	- https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
- A deeper blog post discussion the various timeouts supported by Go
  - https://ieftimov.com/posts/make-resilient-golang-net-http-servers-using-timeouts-deadlines-context-cancellation/

## Getting Started

```bash
go build
# run an echo server with simple testing endpoints
docker run -d -p 8000:80 --name httpbin docker.io/kennethreitz/httpbin:latest

# run the reverse proxy, forwarding all requests to the echo server
# the listening address should have no scheme (server only supports HTTP)
# the target should contain a scheme - the proxy can forward HTTP or HTTPS.
./cohere-reverse-proxy -address 127.0.0.1:8080 -target http://127.0.0.1:8000

# example for https
# ./cohere-reverse-proxy -address 127.0.0.1:8080 -target https://httpbin.org

# in another terminal, try this:
# request /anything on the proxy server, forwarding it to the echo server.
curl http://127.0.0.1:8080/anything
{
  "args": {},
  "data": "",
  "files": {},
  "form": {},
  "headers": {
    "Accept": "*/*",
    "Accept-Encoding": "gzip",
    "Host": "127.0.0.1:8000",
    "User-Agent": "curl/7.81.0",
    "X-Forwarded-Host": "127.0.0.1:8080"
  },
  "json": null,
  "method": "GET",
  "origin": "127.0.0.1",
  "url": "http://127.0.0.1:8080/anything"
}

# drip a response over 4 seconds.
# the response will stream to the client as the server sends it.
curl -N 'http://127.0.0.1:8000/drip?duration=4&numbytes=4&delay=0'
```

This will start the reverse proxy with a listener on 127.0.0.1:8080,
forwarding all client requests to http://127.0.0.1:8000.

The proxy only serves over HTTP (not HTTPS) at this time.
Origin servers may be HTTPS.

## Development

Go is the only dependency to develop the project.

Docker is useful for running test servers.

To contribute, fork this repo, make your changes in a branch, and open a PR.

### Style

Use `gofmt` to format your code before submitting a PR.

```bash
gofmt -w -s .
```

Use `go vet` to check for common issues before submitting a PR.

```bash
go vet ./...
```

### Tests

Use `go test -v ./...` to run all tests with verbose output.

These should pass for PRs to be accepted.

## What's a reverse proxy?

A reverse proxy acts like a normal webserver, but serves no content of its own.
Instead, it forwards all requests to an upstream backend, returning their
responses to the client as if they had been served by the proxy directly.

Reverse proxies may support features like rewriting request URLs and parameters,
authentication prior to allowing an upstream request to proceed, or handling
retries on upstream failures for idempotent requests. They may also support
load balancing, by selecting between many backend instances
when forwarding requests.

## How does it work?

The implementation in this repo is fairly naive.

We create a Go `http.Server`, and create a single handler.
The handler takes an incoming request, clones it, rewrites the URL to
the upstream target, and sends the request like any normal client to the upstream.
It receives the upstream response and copies the the headers and body
back to the client.

While the real implementation uses the Go standard library's `httputil.ReverseProxy`,
the core logic is relatively simple and is shown here:

```go
func proxyFunc(host, scheme string, client *http.Client) func(w http.ResponseWriter, req *http.Request) error {
	return func(w http.ResponseWriter, req *http.Request) error {
		req.Host = host
		req.URL.Host = host
		req.URL.Scheme = scheme
		req.RequestURI = ""

		res, err := client.Do(req)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, err)
		}

		for headerName, headerValues := range res.Header {
			for _, headerValue := range headerValues {
				w.Header().Add(headerName, headerValue)
			}
		}

		w.WriteHeader(res.StatusCode)
		io.Copy(w, res.Body)

		return nil
	}
}
```

This elides many important details, but highlights the core functionality.

Notable missing details Go's stdlib handles nicely for us:
- Some HTTP headers are explicitly hop by hop and should be stripped
  during processing.
- The `io.Copy` above will flush the entire response body, once. For long-lived
  or streaming requests, this may not be ideal. Instead, we configure
  a 10 millisecond periodic flush.
- On failures to connect to the upstream, we should return a 502 Bad Gateway
  response to the client, rather than pass along a connection failure or similar.

# Limitations
- The proxy only supports serving HTTP, not HTTPS.
  - This avoids the need to generate/read certificates, but is insecure.
  - HTTPS can easily be added with a second listener and TLS logic. 
    - A code snippet below shows how to generate a self-signed certificate
      in-memory for usage.
  - The origin server may be HTTPS.
- No load balancing between origin servers.
  - We assume a single origin server for simplicity.
  - A typical reverse proxy may have many origin servers, and wish to load
    balance between them.
- No retries on upstream failures.
  - We assume the origin server is reliable.
  - A typical reverse proxy may wish to retry requests on failures.
- No request rewrites, path matching, etc.
  - We simply forward requests as-is.
- No authentication or authorization.
  - We assume the origin server is public.
  - A reverse proxy may wish to authenticate or authorize requests
    prior to forwarding them.
- No timeout request handler. We may want to gracefully terminate long/slow connections.

# Future Work: Scaling

- Load balancing
  - We could implement load balancing between any number of origin servers.
- Caching
  - We could implement caching of responses, to avoid forwarding requests
    to the origin server for repeated requests.
  - This should only be done for idempotent requests.
- Rate limiting
  - Our server allows unlimited throughput to the origin server.
  - We could handle rate limiting both at the proxy and per-origin level.
- Load testing
  - We could implement load testing of the proxy server, to ensure it
    can handle the load we're sending it.
- Healthchecking
  - We could implement healthchecking of the origin server, to ensure
    we don't send requests to an unhealthy server.
- Multiple host targets
  - We expect a single upstream target host.
  - With a catch-all listener + real DNS/IP configuration, we could receive requests for many
    hosts, and forward them to different origin server groups as needed.
  - Using more sophisticated tooling like BPF would allow us to do live reload
    of listeners and proxy configurations, without terminating any connections.
    - I've previously implemented a TCP proxy using BPF to achieve this behavior
      for another hiring challenge.
    - https://github.com/alexeldeib/fly-platform-challenge/blob/main/NOTES.md
    - https://github.com/alexeldeib/fly-platform-challenge/blob/main/cmd/bpfproxy/main.go

# Future Work: Security

Our proxy has basic protections against common HTTP-based attacks (slow loris, etc),
but is still insecure.

- HTTPS serving endpoint
  - Generate a keypair, add it to http.Server.TLSConfig
	- change Serve() to ServeTLS(), or listen on separate ports.
- TLS or mTLS to origin server
  - We assume our proxy and origin server can freely communicate.
  - In secured production environments, the proxy may terminate TLS and
    re-encrypt using TLS to the origin server.
  - The origin server may also authenticate the client using a client certificate
    for mutual assurance.
- Hop by hop header abuse
  - A user can explicitly indicate a header should be considered hop by hop.
  - If the origin server uses such a header to make a logical decision for the
    response, a user can abuse this to drop the header and return another response.
  - This can be used to perform e.g. cache poisoning attacks.

```go
// example of self-signed cert/CA keypair generation for https serving.
func keypair() ([]byte, []byte, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate rsa key: %s", err)
	}

	keyUsage := x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign
	notBefore := time.Now().Add(time.Hour * -1)
	notAfter := notBefore.Add(365 * 24 * time.Hour)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial number: %s", err)
	}

	template := x509.Certificate{
		IsCA:         true,
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Acme Co"},
		},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, priv.PublicKey, priv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %s", err)
	}

	var certBuf bytes.Buffer
	if err := pem.Encode(&certBuf, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return nil, nil, fmt.Errorf("failed to encode pem certificate: %s", err)
	}

	var keyBuf bytes.Buffer
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal private key: %v", err)
	}
	if err := pem.Encode(&keyBuf, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return nil, nil, fmt.Errorf("failed to write encode pem private key: %v", err)
	}

	return certBuf.Bytes(), keyBuf.Bytes(), nil
}
```