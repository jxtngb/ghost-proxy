package main

import (
	"log"

	"github.com/jxtngb/ghost-proxy/pkg/client"
	"github.com/jxtngb/ghost-proxy/pkg/socks5"
)

func main() {
	log.Println("Ghost Protocol client starting...")

	ghostClient, err := client.FromEnvironment(
		"127.0.0.1:443",
		"localhost",
	)
	if err != nil {
		log.Fatalf("create Ghost client: %v", err)
	}

	server := &socks5.Server{
		Addr: "127.0.0.1:1080",
		Dial: ghostClient.Dial,
	}

	if err := server.Start(); err != nil {
		log.Fatalf("SOCKS5 server stopped: %v", err)
	}
}
