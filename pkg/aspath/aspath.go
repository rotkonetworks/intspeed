//go:build !js

// Package aspath implements a minimal traceroute (UDP probes, raw ICMP
// listener) plus IP→ASN mapping via Team Cymru DNS, producing the AS-level
// path to a host. Raw ICMP needs CAP_NET_RAW (root or setcap).
package aspath

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

type Hop struct {
	TTL int
	IP  string // empty when the hop didn't answer
}

type AS struct {
	ASN  string
	Name string // short (<=10 chars), may be empty
}

// ErrNoPermission is returned when the raw ICMP socket cannot be opened.
var ErrNoPermission = fmt.Errorf("raw ICMP socket requires root or CAP_NET_RAW")

// Trace runs a UDP traceroute toward host. One probe per TTL.
func Trace(ctx context.Context, host string, maxHops int, hopTimeout time.Duration) ([]Hop, error) {
	dst, err := resolve4(host)
	if err != nil {
		return nil, err
	}

	icmpConn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return nil, ErrNoPermission
	}
	defer icmpConn.Close()

	udp, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return nil, err
	}
	defer udp.Close()
	pc := ipv4.NewPacketConn(udp)

	var hops []Hop
	buf := make([]byte, 1500)
	for ttl := 1; ttl <= maxHops; ttl++ {
		if ctx.Err() != nil {
			break
		}
		if err := pc.SetTTL(ttl); err != nil {
			return nil, err
		}
		port := 33434 + ttl
		if _, err := udp.WriteTo([]byte("intspeed-aspath"), &net.UDPAddr{IP: dst, Port: port}); err != nil {
			return nil, err
		}

		hop := Hop{TTL: ttl}
		reached := false
		deadline := time.Now().Add(hopTimeout)
		for time.Now().Before(deadline) {
			icmpConn.SetReadDeadline(deadline)
			n, peer, err := icmpConn.ReadFrom(buf)
			if err != nil {
				break // timeout: unanswered hop
			}
			kind, probeDst, probePort := parseICMP(buf[:n])
			if probeDst == nil || !probeDst.Equal(dst) || probePort != port {
				continue // someone else's ICMP; keep reading until deadline
			}
			hop.IP = peer.String()
			reached = kind == "unreach" || probeDst.Equal(net.ParseIP(peer.String()))
			break
		}
		hops = append(hops, hop)
		if reached {
			break
		}
	}
	return hops, nil
}

// parseICMP extracts the original probe's destination IP and UDP port from a
// time-exceeded / dest-unreachable payload (IP header + first 8 bytes of UDP).
func parseICMP(b []byte) (kind string, dst net.IP, port int) {
	msg, err := icmp.ParseMessage(1, b)
	if err != nil {
		return "", nil, 0
	}
	var data []byte
	switch body := msg.Body.(type) {
	case *icmp.TimeExceeded:
		kind, data = "ttl", body.Data
	case *icmp.DstUnreach:
		kind, data = "unreach", body.Data
	default:
		return "", nil, 0
	}
	if len(data) < 28 { // 20B IP header + 8B UDP header
		return "", nil, 0
	}
	ihl := int(data[0]&0x0f) * 4
	if len(data) < ihl+8 {
		return "", nil, 0
	}
	dst = net.IPv4(data[16], data[17], data[18], data[19])
	port = int(data[ihl+2])<<8 | int(data[ihl+3])
	return kind, dst, port
}

// ASPath collapses hops into the AS-level path. Unanswered and unmapped hops
// (private ranges, IXP fabrics) are skipped; consecutive same-AS hops merge.
func ASPath(hops []Hop) []AS {
	var path []AS
	cache := map[string]string{}
	for _, h := range hops {
		if h.IP == "" {
			continue
		}
		asn := lookupASN(h.IP)
		if asn == "" {
			continue
		}
		if len(path) > 0 && path[len(path)-1].ASN == asn {
			continue
		}
		name, ok := cache[asn]
		if !ok {
			name = lookupASName(asn)
			cache[asn] = name
		}
		path = append(path, AS{ASN: asn, Name: name})
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
