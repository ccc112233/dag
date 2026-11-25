package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ccc112233/dag/internal/api"
	"github.com/ccc112233/dag/internal/config"
	"github.com/ccc112233/dag/internal/pipeline"
	"github.com/ccc112233/dag/internal/plugin"
	"github.com/ccc112233/dag/internal/store"
	"github.com/ccc112233/dag/internal/tscache"
)

func main() {
	cfgPath := flag.String("config", "configs/example.yaml", "config file path")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	cfg, err := config.LoadYAML(*cfgPath)
	if err != nil {
		log.Warn("config load failed, using defaults", "err", err)
		cfg = config.Default()
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Error("open store", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	reg := plugin.NewRegistry()
	registerPlugins(reg)

	eng := pipeline.New(st, reg, log, cfg.Buffer.MaxItems)
	defer eng.Close()

	var ts *tscache.Cache
	if cfg.TSCache.Enabled {
		ts, err = tscache.Open(tscache.Config{
			Path:          cfg.TSCache.Path,
			RetentionDays: cfg.TSCache.RetentionDays,
		})
		if err != nil {
			log.Error("open ts cache", "err", err)
			os.Exit(1)
		}
		defer ts.Close()
		eng.SetTSCache(ts)
		log.Info("ts cache enabled", "path", cfg.TSCache.Path, "retentionDays", cfg.TSCache.RetentionDays)
	}

	eng.RestoreEnabledTasks(context.Background())
	eng.FlushBufferedChannels(context.Background())

	srv := &api.Server{Store: st, Registry: reg, Engine: eng, Token: cfg.APIToken, TS: ts}
	httpSrv := &http.Server{Addr: cfg.Listen, Handler: srv.Handler()}

	go func() {
		log.Info("dap listening", "addr", cfg.Listen)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http server", "err", err)
			os.Exit(1)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
}
