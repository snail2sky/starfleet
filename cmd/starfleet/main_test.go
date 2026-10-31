package main

import "testing"

func TestDefaultAction(t *testing.T) {
	cases := []struct {
		daemon bool
		want   string
	}{
		{false, "tui"}, // 裸跑 = TUI（v1.5 起）
		{true, "serve"}, // -d 显式守护 = 旧行为
	}
	for _, c := range cases {
		if got := defaultAction(c.daemon); got != c.want {
			t.Errorf("defaultAction(%v)=%q want %q", c.daemon, got, c.want)
		}
	}
}
