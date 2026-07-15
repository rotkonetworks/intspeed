//go:build !js

// Package aspath implements a thin mtr: ICMP-echo traceroute with per-hop
// RTT, reverse DNS, and IP→ASN mapping (Team Cymru DNS), producing an
// inspectable looking-glass style path. Raw ICMP needs CAP_NET_RAW.
package aspath

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

type Hop struct {
	TTL    int     `json:"ttl"`
	IP     string  `json:"ip,omitempty"` // empty when the hop didn't answer
	PTR    string  `json:"ptr,omitempty"`
	RTTMs  float64 `json:"rtt_ms,omitempty"`
	ASN    string  `json:"asn,omitempty"`
	ASName string  `json:"as_name,omitempty"`
}

type AS struct {
	ASN  string
	Name string // short (<=10 chars), may be empty
}

// ErrNoPermission is returned when the raw ICMP socket cannot be opened.
var ErrNoPermission = fmt.Errorf("raw ICMP socket requires root or CAP_NET_RAW")

// Trace runs an ICMP-echo traceroute toward host (mtr-style: routers answer
// echo probes far more reliably than UDP). Two probes per TTL, then the hop
// is marked unanswered. Hops are enriched with PTR + ASN before returning.
func Trace(ctx context.Context, host string, maxHops int, hopTimeout time.Duration) ([]Hop, error) {
	dst, err := resolve4(host)
	if err != nil {
		return nil, err
	}

	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return nil, ErrNoPermission
	}
	defer conn.Close()
	pc := conn.IPv4PacketConn()

	id := os.Getpid() & 0xffff
	buf := make([]byte, 1500)
	var hops []Hop

	for ttl := 1; ttl <= maxHops; ttl++ {
		if ctx.Err() != nil {
			break
		}
		if err := pc.SetTTL(ttl); err != nil {
			return nil, err
		}

		hop := Hop{TTL: ttl}
		reached := false
		for attempt := 0; attempt < 2 && hop.IP == ""; attempt++ {
			seq := ttl<<2 | attempt
			wm := icmp.Message{
				Type: ipv4.ICMPTypeEcho,
				Body: &icmp.Echo{ID: id, Seq: seq, Data: []byte("intspeed-aspath")},
			}
			wb, _ := wm.Marshal(nil)
			start := time.Now()
			if _, err := conn.WriteTo(wb, &net.IPAddr{IP: dst}); err != nil {
				return nil, err
			}

			deadline := start.Add(hopTimeout)
			for time.Now().Before(deadline) {
				conn.SetReadDeadline(deadline)
				n, peer, err := conn.ReadFrom(buf)
				if err != nil {
					break // timeout → next attempt
				}
				match, isReply := matchProbe(buf[:n], id, seq)
				if !match {
					continue
				}
				hop.IP = peer.String()
				hop.RTTMs = float64(time.Since(start).Microseconds()) / 1000
				reached = isReply && peer.String() == dst.String()
				break
			}
		}
		hops = append(hops, hop)
		if reached {
			break
		}
	}

	enrich(ctx, hops)
	return hops, nil
}

// matchProbe reports whether an incoming ICMP packet answers our echo probe
// (either an echo reply from the destination, or a time-exceeded /
// dest-unreachable quoting our probe), and whether it was the final reply.
func matchProbe(b []byte, id, seq int) (match, isReply bool) {
	msg, err := icmp.ParseMessage(1, b)
	if err != nil {
		return false, false
	}
	switch body := msg.Body.(type) {
	case *icmp.Echo:
		return msg.Type == ipv4.ICMPTypeEchoReply && body.ID == id && body.Seq == seq, true
	case *icmp.TimeExceeded:
		return quotedEchoMatches(body.Data, id, seq), false
	case *icmp.DstUnreach:
		return quotedEchoMatches(body.Data, id, seq), false
	}
	return false, false
}

// quotedEchoMatches digs the original echo header out of an ICMP error
// payload (IP header + first 8 bytes of the offending packet).
func quotedEchoMatches(data []byte, id, seq int) bool {
	if len(data) < 20 {
		return false
	}
	ihl := int(data[0]&0x0f) * 4
	if len(data) < ihl+8 || data[ihl] != 8 { // type 8 = echo request
		return false
	}
	qid := int(data[ihl+4])<<8 | int(data[ihl+5])
	qseq := int(data[ihl+6])<<8 | int(data[ihl+7])
	return qid == id && qseq == seq
}

// enrich adds PTR names and ASN info to answered hops.
func enrich(ctx context.Context, hops []Hop) {
	nameCache := map[string]string{}
	for i := range hops {
		if hops[i].IP == "" {
			continue
		}
		hops[i].PTR = lookupPTR(ctx, hops[i].IP)
		asn := lookupASN(hops[i].IP)
		if asn == "" {
			continue
		}
		hops[i].ASN = asn
		name, ok := nameCache[asn]
		if !ok {
			name = lookupASName(asn)
			nameCache[asn] = name
		}
		hops[i].ASName = name
	}
}

// ASPath collapses hops into the AS-level path: every AS traversed, in
// order. Unanswered and unmapped hops (private ranges, IXP fabrics) are
// skipped; consecutive same-AS hops merge.
func ASPath(hops []Hop) []AS {
	var path []AS
	for _, h := range hops {
		if h.ASN == "" {
			continue
		}
		if len(path) > 0 && path[len(path)-1].ASN == h.ASN {
			continue
		}
		path = append(path, AS{ASN: h.ASN, Name: h.ASName})
	}
	return path
}

// PeeringDBURL returns the search URL for an ASN, e.g.
// https://www.peeringdb.com/search?q=as142108
func PeeringDBURL(asn string) string {
	return "https://www.peeringdb.com/search?q=as" + asn
}

func resolve4(host string) (net.IP, error) {
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			return v4, nil
		}
	}
	return nil, fmt.Errorf("no IPv4 address for %s", host)
}

func lookupPTR(ctx context.Context, ip string) string {
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	names, err := net.DefaultResolver.LookupAddr(ctx, ip)
	if err != nil || len(names) == 0 {
		return ""
	}
	return strings.TrimSuffix(names[0], ".")
}

func lookupASN(ip string) string {
	p := net.ParseIP(ip).To4()
	if p == nil || isPrivate(p) {
		return ""
	}
	q := fmt.Sprintf("%d.%d.%d.%d.origin.asn.cymru.com", p[3], p[2], p[1], p[0])
	txts, err := net.LookupTXT(q)
	if err != nil || len(txts) == 0 {
		return ""
	}
	fields := strings.Fields(strings.Split(txts[0], "|")[0])
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// lookupASName returns a short lowercase name (<=10 chars) for an ASN.
func lookupASName(asn string) string {
	txts, err := net.LookupTXT("AS" + asn + ".asn.cymru.com")
	if err != nil || len(txts) == 0 {
		return ""
	}
	parts := strings.Split(txts[0], "|")
	if len(parts) < 5 {
		return ""
	}
	name := strings.TrimSpace(parts[4])
	if i := strings.LastIndex(name, ","); i > 0 {
		name = name[:i]
	}
	name = strings.ToLower(name)
	if i := strings.Index(name, " "); i >= 3 {
		name = name[:i]
	}
	name = strings.Trim(name, "-_.")
	if len(name) > 10 {
		name = name[:10]
	}
	return name
}

func isPrivate(ip net.IP) bool {
	return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()
}
