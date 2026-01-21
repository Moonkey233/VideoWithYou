package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"videowithyou/v2/server/internal/server"
)

func main() {
	addr := flag.String("addr", ":9012", "listen address")
	path := flag.String("path", "/ws", "websocket path")
	hostIdleTimeoutSec := flag.Int("host_idle_timeout_sec", 600, "close room if host idle (seconds)")
	flag.Parse()

	srv := server.NewServer(log.Default())
	if *hostIdleTimeoutSec > 0 {
		srv.SetHostIdleTimeout(time.Duration(*hostIdleTimeoutSec) * time.Second)
	}
	http.HandleFunc(*path, srv.HandleWS)

    log.Printf("server listening on %s%s", *addr, *path)
    if err := http.ListenAndServe(*addr, nil); err != nil {
        log.Fatalf("server stopped: %v", err)
    }
}
