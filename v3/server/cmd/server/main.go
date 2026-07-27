package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"videowithyou/v3/internal/roomserver"
)

func main() {
	addr := flag.String("addr", ":21314", "listen address")
	path := flag.String("path", "/ws", "websocket path")
	accessToken := flag.String("access_token", "", "optional client access token")
	reconnectGraceSec := flag.Int("reconnect_grace_sec", 30, "abnormal disconnect resume window")
	hostIdleTimeoutSec := flag.Int("host_idle_timeout_sec", 600, "close room if host is idle")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	room := roomserver.New(roomserver.Config{
		AccessToken:     *accessToken,
		ReconnectGrace:  time.Duration(*reconnectGraceSec) * time.Second,
		HostIdleTimeout: time.Duration(*hostIdleTimeoutSec) * time.Second,
	}, log.Default())
	room.Start(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc(*path, room.HandleWS)
	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	log.Printf("[服务端] 独立兼容服务监听 ws://%s%s", *addr, *path)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("[服务端] 已停止 error=%q", err)
	}
}
