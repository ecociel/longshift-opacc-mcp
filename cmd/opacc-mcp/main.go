package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/ecociel/longshift-opacc-mcp/internal/config"
	"github.com/ecociel/longshift-opacc-mcp/internal/server"
	"github.com/ecociel/longshift-opacc-mcp/internal/world"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("opacc-mcp: ")

	cfg, err := config.FromEnv()
	if err != nil {
		log.Fatal(err)
	}
	w := world.New(world.Options{
		Seed:      cfg.Seed,
		Window:    cfg.Window,
		TimeScale: cfg.TimeScale,
		Now:       cfg.Now,
	})
	srv := server.New(cfg, w)

	runCtx, stopRun := context.WithCancel(context.Background())
	defer stopRun()
	go w.Run(runCtx, cfg.Tick)

	if cfg.Transport == "stdio" {
		if err := srv.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			log.Fatal(err)
		}
		return
	}

	httpSrv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.HTTPHandler(srv, w),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		ln, err := net.Listen("tcp", cfg.Addr)
		if err != nil {
			errCh <- err
			return
		}
		log.Printf("streamable HTTP MCP on %s (endpoint /mcp, health /healthz)", ln.Addr())
		errCh <- httpSrv.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		stopRun()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	case err := <-errCh:
		stopRun()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}
}
