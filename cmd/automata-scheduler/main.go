package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/shaiso/Automata/internal/repo"
)

const schedLockKey int64 = 424242

func main() {
	// graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// DB pool
	pool, err := repo.NewPool(ctx)
	if err != nil {
		log.Fatalf("[scheduler] db connect: %v", err)
	}
	defer pool.Close()
	log.Printf("[scheduler] db connected")

	// HTTP mux: /healthz + /metrics
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.Handle("/metrics", promhttp.Handler())

	// scheduler loop
	go func() {
		tk := time.NewTicker(1 * time.Second)
		defer tk.Stop()

		var hasLock bool
		defer func() {
			if hasLock {
				_, _ = pool.Exec(context.Background(), "select pg_advisory_unlock($1)", schedLockKey)
			}
		}()

		for {
			select {
			case t := <-tk.C:
				// пытаемся стать лидером (или подтвердить лидерство)
				var ok bool
				if !hasLock {
					if err := pool.QueryRow(ctx, "select pg_try_advisory_lock($1)", schedLockKey).Scan(&ok); err != nil {
						log.Printf("[scheduler] lock err: %v", err)
						continue
					}
					hasLock = ok
				}

				if !hasLock {
					// не лидер — пропускаем тик
					continue
				}

				// лидер выполняет логику тика
				log.Printf("[scheduler] tick %s", t.Format(time.RFC3339))
				// TODO: тут будет выборка due schedules -> создание runs -> bump next_due_at

			case <-ctx.Done():
				return
			}
		}
	}()

	// serve
	port := ":8081"
	if v := os.Getenv("SCHED_PORT"); v != "" {
		port = ":" + v
	}
	log.Printf("[scheduler] listening on %s", port)
	if err := http.ListenAndServe(port, mux); err != nil {
		log.Printf("[scheduler] http error: %v", err)
		cancel()
	}
}
