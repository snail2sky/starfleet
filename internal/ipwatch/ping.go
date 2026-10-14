package ipwatch

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"time"
)

// PingResult is the outcome of probing one IP once.
type PingResult struct {
	IP    string
	Alive bool
	RTT   time.Duration // zero when dead or unparsed
}

var (
	timeRe = regexp.MustCompile(`time[=<]([\d.]+)\s*ms`)
	// Debian/BusyBox: "1 packets transmitted, 1 received"
	pktRe = regexp.MustCompile(`(\d+) packets transmitted, (\d+) (?:packets )?received`)
)

// PingSweep probes every IP in targets with up to workers parallel pings.
// Each probe is `ping -c1 -W1` (1s deadline) — works unprivileged on
// Linux (ping carries cap_net_raw or is setuid).
func PingSweep(ctx context.Context, targets []string, workers int) []PingResult {
	if workers < 1 {
		workers = 32
	}
	results := make([]PingResult, len(targets))
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for i, ip := range targets {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, ip string) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = pingOne(ctx, ip)
		}(i, ip)
	}
	wg.Wait()
	return results
}

func pingOne(ctx context.Context, ip string) PingResult {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ping", "-c", "1", "-W", "1", ip)
	out, err := cmd.CombinedOutput()
	res := PingResult{IP: ip}
	text := string(out)
	if m := pktRe.FindStringSubmatch(text); m != nil {
		sent, recv := m[1], m[2]
		if sent != recv {
			return res // lost
		}
	} else if err != nil {
		return res
	}
	res.Alive = true
	if m := timeRe.FindStringSubmatch(text); m != nil {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			res.RTT = time.Duration(v * float64(time.Millisecond))
		}
	}
	return res
}

// ExpandSubnet returns every usable host address of a /24-ish IPv4 network
// (e.g. 192.168.1.0/24 -> .1 ... .254). Networks larger than /24 are
// rejected to keep scans bounded.
func ExpandSubnet(cidr string) ([]string, error) {
	ip, bits, err := parseCIDR(cidr)
	if err != nil {
		return nil, err
	}
	if bits < 24 {
		return nil, fmt.Errorf("subnet %s too large (want /24 or smaller host range)", cidr)
	}
	base := ip
	var out []string
	for i := 1; i < hostCount(bits)-1; i++ {
		out = append(out, incIP(base, i))
	}
	return out, nil
}

// parseCIDR parses "a.b.c.d/n" into the network base and prefix length.
func parseCIDR(s string) (string, int, error) {
	var a, b, c, d, n int
	if _, err := fmt.Sscanf(s, "%d.%d.%d.%d/%d", &a, &b, &c, &d, &n); err != nil {
		return "", 0, fmt.Errorf("bad subnet %q (want e.g. 192.168.1.0/24)", s)
	}
	for _, v := range []int{a, b, c, d} {
		if v < 0 || v > 255 {
			return "", 0, fmt.Errorf("bad subnet %q", s)
		}
	}
	if n < 1 || n > 32 {
		return "", 0, fmt.Errorf("bad prefix in %q", s)
	}
	// Zero the host bits to get the network base.
	mask := ^uint32(0) << (32 - n)
	val := uint32(a)<<24 | uint32(b)<<16 | uint32(c)<<8 | uint32(d)
	val &= mask
	return fmt.Sprintf("%d.%d.%d.%d", val>>24, val>>16&0xff, val>>8&0xff, val&0xff), n, nil
}

func hostCount(prefix int) int { return 1 << (32 - prefix) }

// incIP returns base+i as dotted quad.
func incIP(base string, i int) string {
	var a, b, c, d int
	_, _ = fmt.Sscanf(base, "%d.%d.%d.%d", &a, &b, &c, &d)
	v := uint32(a)<<24 | uint32(b)<<16 | uint32(c)<<8 | uint32(d)
	v += uint32(i)
	return fmt.Sprintf("%d.%d.%d.%d", v>>24, v>>16&0xff, v>>8&0xff, v&0xff)
}
