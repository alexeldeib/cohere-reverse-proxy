package main

import (
	"flag"
	"log"
	"net/url"
	"os"

	"github.com/alexeldeib/cohere-reverse-proxy/internal"
)

func main() {
	var (
		address   string
		targetURL string
	)

	flag.StringVar(&address, "address", "127.0.0.1:8001", "address for reverse proxy to listen on")
	flag.StringVar(&targetURL, "target", "http://127.0.0.1:8000", "origin server to which the proxy should forward requests")

	flag.Parse()

	url, err := url.Parse(targetURL)
	if err != nil {
		log.Fatalln(err)
	}

	srv := internal.NewServer(url)

	log.Println("Starting up the server")

	if err := srv.ListenAndServe(address); err != nil {
		log.Println(err)
		os.Exit(1)
	}

	log.Println("Server stopped cleanly")
}
