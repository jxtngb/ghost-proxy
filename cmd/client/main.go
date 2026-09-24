package main

import (
	"log"

	"github.com/jxtngb/ghost-proxy/pkg/socks5"
)

func main() {
	log.Println("Ghost Protocol client starting...")

	server := &socks5.Server{
		Addr: "127.0.0.1:1080",
	}

	if err := server.Start(); err != nil {
		log.Fatalf("SOCKS5 server stopped: %v", err)
	}
}
