package main

import (
    "flag"
    "log"
    "net/http"

    "videowithyou/v2/server/internal/server"
)

func main() {
    addr := flag.String("addr", ":2333", "listen address")
    path := flag.String("path", "/ws", "websocket path")
    flag.Parse()

    srv := server.NewServer(log.Default())
    http.HandleFunc(*path, srv.HandleWS)

    log.Printf("server listening on %s%s", *addr, *path)
    if err := http.ListenAndServe(*addr, nil); err != nil {
        log.Fatalf("server stopped: %v", err)
    }
}