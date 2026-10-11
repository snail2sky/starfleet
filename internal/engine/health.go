package engine

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/snail2sky/starfleet/internal/config"
	"github.com/snail2sky/starfleet/internal/event"
)

// HealthChecker monitors process health and triggers restarts.
type HealthChecker struct {
	mu      sync.RWMutex
	checks  map[string]*healthCheck
	bus     *event.Bus
	stopCh  chan struct{}
	logger  *slog.Logger
}

type healthCheck struct {
	process   string
	config    config.HealthCheckConfig
	failCount int
}

// NewHealthChecker creates a new health checker.
func NewHealthChecker(bus *event.Bus) *HealthChecker {
	return &HealthChecker{
		checks: make(map[string]*healthCheck),
		bus:    bus,
		stopCh: make(chan struct{}),
		logger: slog.With("component", "health"),
	}
}

// Register registers a health check for a process.
func (hc *HealthChecker) Register(process string, cfg config.HealthCheckConfig) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.checks[process] = &healthCheck{
		process: process,
		config:  cfg,
	}
	hc.logger.Info("health check registered", "process", process, "type", cfg.Type)
}

// Unregister removes a health check.
func (hc *HealthChecker) Unregister(process string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	delete(hc.checks, process)
}

// Start begins the health check loop.
func (hc *HealthChecker) Start() {
	go hc.run()
	hc.logger.Info("health checker started")
}

// Stop stops the health checker.
func (hc *HealthChecker) Stop() {
	close(hc.stopCh)
}

func (hc *HealthChecker) run() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			hc.checkAll()
		case <-hc.stopCh:
			return
		}
	}
}

func (hc *HealthChecker) checkAll() {
	hc.mu.RLock()
	checks := make([]*healthCheck, 0, len(hc.checks))
	for _, c := range hc.checks {
		checks = append(checks, c)
	}
	hc.mu.RUnlock()

	for _, c := range checks {
		go hc.check(c)
	}
}

func (hc *HealthChecker) check(c *healthCheck) {
	timeout := c.config.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	var err error
	switch c.config.Type {
	case "http":
		err = checkHTTP(c.config.URL, timeout)
	case "tcp":
		err = checkTCP(c.config.Address, timeout)
	default: // "process" - just check if process is running
		return
	}

	hc.mu.Lock()
	defer hc.mu.Unlock()

	if err != nil {
		c.failCount++
		hc.logger.Warn("health check failed",
			"process", c.process,
			"fail_count", c.failCount,
			"error", err,
		)
		if c.failCount >= c.config.Retries {
			hc.bus.Publish(event.Event{
				Type:    event.EventHealthCheckFail,
				Process: c.process,
				Data: map[string]interface{}{
					"fail_count": c.failCount,
					"error":      err.Error(),
				},
			})
		}
	} else {
		if c.failCount > 0 {
			hc.logger.Info("health check recovered", "process", c.process)
		}
		c.failCount = 0
	}
}

func checkHTTP(url string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("health check returned %d", resp.StatusCode)
	}
	return nil
}

func checkTCP(address string, timeout time.Duration) error {
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}