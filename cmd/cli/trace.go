package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/rotkonetworks/intspeed/pkg/aspath"
	"github.com/rotkonetworks/intspeed/pkg/endpoints"
	"github.com/spf13/cobra"
)

var traceEndpoint string

func newTraceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trace [location|all]",
		Short: "Thin mtr: per-hop RTT, reverse DNS and AS path to a location's endpoint",
		Args:  cobra.MaximumNArgs(1),
		Run:   runTrace,
	}
	cmd.Flags().StringVar(&traceEndpoint, "endpoint", "", "Endpoint name within the location (default: first)")
	return cmd
}

func runTrace(cmd *cobra.Command, args []string) {
	reg, err := endpoints.Load()
	if err != nil {
		log.Fatalf("load endpoint registry: %v", err)
	}

	target := "all"
	if len(args) > 0 {
		target = args[0]
	}

	var locs []endpoints.LocationEndpoints
	if strings.EqualFold(target, "all") {
		locs = reg.Locations
	} else {
		loc := reg.ForLocation(canonicalName(reg, target))
		if loc == nil {
			log.Fatalf("unknown location %q — see `intspeed locations`", target)
		}
		locs = []endpoints.LocationEndpoints{*loc}
	}

	for i, loc := range locs {
		if i > 0 {
			fmt.Println()
		}
		traceLocation(reg, loc)
	}
}

func canonicalName(reg *endpoints.Registry, name string) string {
	for _, l := range reg.Locations {
		if strings.EqualFold(l.Name, name) {
			return l.Name
		}
	}
	return name
}

func traceLocation(reg *endpoints.Registry, loc endpoints.LocationEndpoints) {
	ep := loc.Endpoints[0]
	if traceEndpoint != "" {
		found := false
		for _, e := range loc.Endpoints {
			if strings.EqualFold(e.Name, traceEndpoint) {
				ep, found = e, true
				break
			}
		}
		if !found {
			log.Fatalf("no endpoint %q in %s", traceEndpoint, loc.Name)
		}
	}
	host := endpointHost(reg, loc.Name, ep.Name)
	if host == "" {
		fmt.Printf("%s: no resolvable endpoint\n", loc.Name)
		return
	}

	fmt.Printf("trace to %s (%s · %s)\n", host, loc.Name, ep.Name)
	fmt.Printf("%4s  %-15s %9s  %-22s %s\n", "TTL", "IP", "RTT", "AS", "PTR")
	fmt.Println(strings.Repeat("─", 96))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	hops, err := aspath.Trace(ctx, host, 30, 800*time.Millisecond)
	if err == aspath.ErrNoPermission {
		fmt.Println("raw ICMP needs privileges — run as root or:")
		fmt.Println("  sudo setcap cap_net_raw+ep $(which intspeed)")
		return
	}
	if err != nil {
		fmt.Printf("trace failed: %v\n", err)
		return
	}

	for _, h := range hops {
		if h.IP == "" {
			fmt.Printf("%4d  %-15s %9s\n", h.TTL, "*", "")
			continue
		}
		as := ""
		if h.ASN != "" {
			label := h.ASName
			if label == "" {
				label = "as" + h.ASN
			}
			as = osc8(aspath.PeeringDBURL(h.ASN), fmt.Sprintf("%-10s", label)) + fmt.Sprintf(" %-11s", "("+h.ASN+")")
		} else {
			as = fmt.Sprintf("%-22s", "—")
		}
		fmt.Printf("%4d  %-15s %8.1fms  %s %s\n", h.TTL, h.IP, h.RTTMs, as, h.PTR)
	}

	path := aspath.ASPath(hops)
	if len(path) > 0 {
		segs := make([]string, len(path))
		for i, as := range path {
			label := as.Name
			if label == "" {
				label = "as" + as.ASN
			}
			segs[i] = osc8(aspath.PeeringDBURL(as.ASN), label)
		}
		fmt.Printf("\nas-path: %s\n", strings.Join(segs, " → "))
	}
}
