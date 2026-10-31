package notify

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/snail2sky/starfleet/internal/config"
	"github.com/snail2sky/starfleet/internal/event"
)

type received struct {
	Event     string                 `json:"event"`
	Process   string                 `json:"process"`
	Timestamp string                 `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

func startReceiver(t *testing.T, status int) (*httptest.Server, *sync.Mutex, *[]received) {
	t.Helper()
	var mu sync.Mutex
	var got []received
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rcv received
		if err := json.NewDecoder(r.Body).Decode(&rcv); err != nil {
			t.Errorf("payload 不是 JSON: %v", err)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type=%q", ct)
		}
		mu.Lock()
		got = append(got, rcv)
		mu.Unlock()
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv, &mu, &got // 返回指针：闭包 append 的是原变量，按值返回会拿到空副本
}

// waitSubscriberReady 等订阅协程进入接收循环（Start 到 run 就绪有微秒级窗口，
// 事件总线是非阻塞投递，过早发布会丢）。
func waitSubscriberReady(t *testing.T, n *Notifier) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if n.sub != nil {
			time.Sleep(30 * time.Millisecond)
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("订阅未就绪")
}

func waitFor(t *testing.T, mu *sync.Mutex, got *[]received, n int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		cur := len(*got)
		mu.Unlock()
		if cur >= n {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("超时：只收到 %d 条，期望 %d 条", len(*got), n)
}

func TestWebhookDeliversSubscribedEvents(t *testing.T) {
	srv, mu, got := startReceiver(t, 200)
	bus := event.NewBus(8)
	defer bus.Close()

	n := NewNotifier(config.NotificationsConfig{
		Webhook: config.WebhookConfig{
			Enabled: true,
			URL:     srv.URL,
			Events:  []string{"process_started", "process_failed"},
		},
	}, bus)
	n.Start()
	defer n.Stop()
	waitSubscriberReady(t, n)

	bus.Publish(event.Event{Type: "process_started", Process: "nginx", Data: map[string]interface{}{"pid": 123}})
	waitFor(t, mu, got, 1)

	mu.Lock()
	defer mu.Unlock()
	g := (*got)[0]
	if g.Event != "process_started" || g.Process != "nginx" {
		t.Errorf("payload=%+v", g)
	}
	if _, err := time.Parse(time.RFC3339, g.Timestamp); err != nil {
		t.Errorf("timestamp 应为 RFC3339: %q", g.Timestamp)
	}
	if pid, ok := g.Data["pid"].(float64); !ok || pid != 123 {
		t.Errorf("data=%v", g.Data)
	}
}

func TestWebhookIgnoresUnsubscribedEvents(t *testing.T) {
	srv, mu, got := startReceiver(t, 200)
	bus := event.NewBus(8)
	defer bus.Close()

	n := NewNotifier(config.NotificationsConfig{
		Webhook: config.WebhookConfig{Enabled: true, URL: srv.URL, Events: []string{"process_started"}},
	}, bus)
	n.Start()
	defer n.Stop()
	waitSubscriberReady(t, n)

	bus.Publish(event.Event{Type: "schedule_completed", Process: "backup"})
	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if len(*got) != 0 {
		t.Errorf("未订阅的事件不应投递: %+v", *got)
	}
}

func TestWebhookDisabledSendsNothing(t *testing.T) {
	srv, mu, got := startReceiver(t, 200)
	bus := event.NewBus(8)
	defer bus.Close()

	n := NewNotifier(config.NotificationsConfig{
		Webhook: config.WebhookConfig{Enabled: false, URL: srv.URL, Events: []string{"process_started"}},
	}, bus)
	n.Start()
	defer n.Stop()
	// disabled 时 Start 直接返回（无订阅协程），不等 ready

	bus.Publish(event.Event{Type: "process_started", Process: "x"})
	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if len(*got) != 0 {
		t.Errorf("disabled 时不应投递")
	}
}

func TestWebhookServerErrorDoesNotPanic(t *testing.T) {
	srv, mu, got := startReceiver(t, 500)
	bus := event.NewBus(8)
	defer bus.Close()

	n := NewNotifier(config.NotificationsConfig{
		Webhook: config.WebhookConfig{Enabled: true, URL: srv.URL, Events: []string{"process_started"}},
	}, bus)
	n.Start()
	defer n.Stop()
	waitSubscriberReady(t, n)

	bus.Publish(event.Event{Type: "process_started", Process: "x"})
	waitFor(t, mu, got, 1) // 服务端收到即可；5xx 只记日志不崩
}

func TestWebhookUnreachableDoesNotPanic(t *testing.T) {
	bus := event.NewBus(8)
	defer bus.Close()
	n := NewNotifier(config.NotificationsConfig{
		Webhook: config.WebhookConfig{Enabled: true, URL: "http://127.0.0.1:1/hook", Events: []string{"process_started"}},
	}, bus)
	n.Start()
	defer n.Stop()
	waitSubscriberReady(t, n)
	bus.Publish(event.Event{Type: "process_started", Process: "x"})
	time.Sleep(200 * time.Millisecond) // 不 panic 即通过
}

func TestSendTest(t *testing.T) {
	srv, _, _ := startReceiver(t, 200)
	if err := SendTest(srv.URL); err != nil {
		t.Errorf("SendTest 成功路径: %v", err)
	}
	srv500, _, _ := startReceiver(t, 500)
	if err := SendTest(srv500.URL); err == nil {
		t.Errorf("5xx 应报错")
	}
	if err := SendTest("http://127.0.0.1:1"); err == nil {
		t.Errorf("不可达应报错")
	}
	if err := SendTest("://bad-url"); err == nil {
		t.Errorf("坏 URL 应报错")
	}
}
