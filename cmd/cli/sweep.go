package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rotkonetworks/intspeed/pkg/endpoints"
	"github.com/rotkonetworks/intspeed/pkg/engine"
	"github.com/spf13/cobra"
)

var (
	sweepDownloadMB int64
	sweepUploadMB   int64
	sweepPings      int
	sweepLocations  string
	sweepJSON       bool
)

func newSweepCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sweep",
		Short: "Test all global locations against the verified endpoint registry",
		Run:   runSweep,
	}
	cmd.Flags().Int64Var(&sweepDownloadMB, "download-mb", 15, "Download size per location (MB)")
	cmd.Flags().Int64Var(&sweepUploadMB, "upload-mb", 6, "Upload size per location (MB)")
	cmd.Flags().IntVar(&sweepPings, "pings", 4, "Latency samples per endpoint")
	cmd.Flags().StringVar(&sweepLocations, "locations", "", "Comma-separated subset of locations (default: all)")
	cmd.Flags().BoolVar(&sweepJSON, "json", false, "Print raw JSON results to stdout")
	return cmd
}

func runSweep(cmd *cobra.Command, args []string) {
	reg, err := endpoints.Load()
	if err != nil {
		log.Fatalf("load endpoint registry: %v", err)
	}

	opts := engine.Options{
		DownloadBytes: sweepDownloadMB * 1_000_000,
		UploadBytes:   sweepUploadMB * 1_000_000,
		PingCount:     sweepPings,
	}
	if sweepLocations != "" {
		opts.Locations = strings.Split(sweepLocations, ",")
	}

	if !sweepJSON {
		fmt.Printf("🌍 intspeed sweep — registry verified %s\n\n", reg.Verified)
	}

	results := engine.Sweep(context.Background(), reg, opts, func(p engine.Progress) {
		if sweepJSON {
			return
		}
		switch p.Type {
		case "location_start":
			fmt.Printf("[%d/%d] %s\n", p.Index, p.Total, p.Location)
		case "download":
			fmt.Printf("        ↓ %.1f Mbps via %s\n", p.Value, p.Endpoint)
		case "upload":
			fmt.Printf("        ↑ %.1f Mbps via %s\n", p.Value, p.Endpoint)
		case "location_done":
			if p.Error != "" {
				fmt.Printf("        ⚠️  %s\n", p.Error)
			}
		}
	})

	if sweepJSON {
		json.NewEncoder(os.Stdout).Encode(results)
	} else {
		printSweepTable(results)
	}

	if err := os.MkdirAll(outputDir, 0755); err == nil {
		out := struct {
			Timestamp time.Time               `json:"timestamp"`
			Results   []engine.LocationResult `json:"results"`
		}{time.Now(), results}
		if data, err := json.MarshalIndent(out, "", "  "); err == nil {
			file := filepath.Join(outputDir, fmt.Sprintf("sweep_%s.json", time.Now().Format("2006-01-02_15-04-05")))
			os.WriteFile(file, data, 0644)
			os.WriteFile(filepath.Join(outputDir, "sweep_latest.json"), data, 0644)
			if !sweepJSON {
				fmt.Printf("\n📊 Results saved: %s\n", file)
			}
		}
	}
}

func printSweepTable(results []engine.LocationResult) {
	fmt.Printf("\n%-13s %9s %8s %10s %10s   %s\n", "LOCATION", "PING", "JITTER", "DOWN", "UP", "VIA")
	fmt.Println(strings.Repeat("─", 78))
	for _, r := range results {
		if r.LatencyMs == 0 {
			fmt.Printf("%-13s %s\n", r.Location, "unreachable: "+r.Error)
			continue
		}
		fmt.Printf("%-13s %7.1fms %6.1fms %7.1f Mb %7.1f Mb   %s\n",
			r.Location, r.LatencyMs, r.JitterMs, r.DownloadMbps, r.UploadMbps, r.DownloadVia)
	}
}
