// Package notify provides external notification support.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/snail-sky/starfleet/internal/config"
	"github.com/snail-sky/starfleet/internal/event"
)

// Notifier sends notifications for events.
type Notifier struct {
	cfg    config.NotificationsConfig
	bus    *event.Bus
	sub    *event.Subscriber
	stopCh chan struct{}
	logger *slog.Logger
}

// NewNotifier creates a new notifier.
func NewNotifier(cfg config.NotificationsConfig, bus *event.Bus) *Notifier {
	return &Notifier{
		cfg:    cfg,
		bus:    bus,
		stopCh: make(chan struct{}),
		logger: slog.With("component", "notifier"),
	}
}

// Start begins listening for events and sending notifications.
func (n *Notifier) Start() {
	if !n.cfg.Webhook.Enabled {
		n.logger.Info("notifications disabled")
		return
	}
	events := make([]event.EventType, 0, len(n.cfg.Webhook.Events))
	for _, e := range n.cfg.Webhook.Events {
		events = append(events, event.EventType(e))
	}
	n.sub = n.bus.Subscribe(events...)
	go n.run()
	n.logger.Info("notifier started", "webhook_url", n.cfg.Webhook.URL)
}

// Stop stops the notifier.
func (n *Notifier) Stop() {
	close(n.stopCh)
	if n.sub != nil {
		n.bus.Unsubscribe(n.sub.ID)
	}
}

func (n *Notifier) run() {
	for {
		select {
		case ev, ok := <-n.sub.Ch:
			if !ok {
				return
			}
			n.sendWebhook(ev)
		case <-n.stopCh:
			return
		}
	}
}

func (n *Notifier) sendWebhook(ev event.Event) {
	payload := map[string]interface{}{
		"event":     ev.Type,
		"process":   ev.Process,
		"timestamp": ev.Timestamp.Format(time.RFC3339),
		"data":      ev.Data,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		n.logger.Error("failed to marshal webhook payload", "error", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", n.cfg.Webhook.URL, bytes.NewReader(body))
	if err != nil {
		n.logger.Error("failed to create webhook request", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Starfleet/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		n.logger.Error("webhook request failed", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		n.logger.Error("webhook returned error", "status", resp.StatusCode)
		return
	}
	n.logger.Info("webhook sent", "event", ev.Type, "process", ev.Process)
}

// SendTest sends a test webhook notification.
func SendTest(url string) error {
	payload := map[string]interface{}{
		"event":     "test",
		"process":   "starfleet",
		"timestamp": time.Now().Format(time.RFC3339),
		"data":      map[string]string{"message": "test notification"},
	}
	body, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned %d", resp.StatusCode)
	}
	return nil
}