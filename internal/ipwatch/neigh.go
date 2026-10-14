package ipwatch

import (
	"bufio"
	"strings"
)

// activeNeighStates are neighbor-table states that indicate the entry was
// verified recently (STALE/DELAY linger after a device is gone, so they
// are kept for MAC enrichment only, never counted as "online").
var activeNeighStates = map[string]bool{
	"REACHABLE": true,
	"DELAY":     true,
	"PROBE":     true,
}

// ParseNeigh parses `ip -4 neigh` output.
//
//	192.168.1.5 dev wlan0 lladdr aa:bb:cc:dd:ee:ff REACHABLE
//	192.168.1.9 dev wlan0  STALE
//
// Returns all ip->mac mappings (for enrichment) and the subset of IPs in
// an actively-verified state (usable as online evidence).
func ParseNeigh(out string) (macs map[string]string, active map[string]bool) {
	macs = make(map[string]string)
	active = make(map[string]bool)
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 {
			continue
		}
		ip := fields[0]
		if !looksLikeIPv4(ip) {
			continue
		}
		state := fields[len(fields)-1]
		mac := ""
		for i := 0; i+1 < len(fields); i++ {
			if fields[i] == "lladdr" {
				mac = fields[i+1]
				break
			}
		}
		if mac != "" {
			macs[ip] = mac
		}
		if activeNeighStates[state] {
			active[ip] = true
		}
	}
	return macs, active
}

// looksLikeIPv4 is a cheap check (no net.ParseIP allocation in hot path).
func looksLikeIPv4(s string) bool {
	dots := 0
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r == '.':
			dots++
		default:
			return false
		}
	}
	return dots == 3
}
