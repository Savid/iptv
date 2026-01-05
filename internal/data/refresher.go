package data

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Refresher periodically refreshes M3U and EPG data.
type Refresher struct {
	log      logrus.FieldLogger
	fetcher  *Fetcher
	interval time.Duration

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewRefresher creates a new data refresher.
func NewRefresher(log logrus.FieldLogger, fetcher *Fetcher, interval time.Duration) *Refresher {
	return &Refresher{
		log:      log.WithField("component", "refresher"),
		fetcher:  fetcher,
		interval: interval,
	}
}

// Start begins the refresh loop.
func (r *Refresher) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cancel != nil {
		return nil // Already running
	}

	refreshCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	r.done = make(chan struct{})

	go r.run(refreshCtx)

	r.log.WithField("interval", r.interval).Info("Data refresher started")

	return nil
}

// Stop stops the refresh loop.
func (r *Refresher) Stop() error {
	r.mu.Lock()
	cancel := r.cancel
	done := r.done
	r.cancel = nil
	r.done = nil
	r.mu.Unlock()

	if cancel != nil {
		cancel()

		if done != nil {
			<-done
		}
	}

	r.log.Info("Data refresher stopped")

	return nil
}

func (r *Refresher) run(ctx context.Context) {
	defer close(r.done)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.refresh(ctx)
		}
	}
}

func (r *Refresher) refresh(ctx context.Context) {
	r.log.Info("Refreshing data")

	if err := r.fetcher.FetchAll(ctx); err != nil {
		r.log.WithError(err).Error("Failed to refresh data")

		return
	}

	r.log.Info("Data refreshed successfully")
}
