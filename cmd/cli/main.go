package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rotkonetworks/intspeed/pkg/locations"
	"github.com/rotkonetworks/intspeed/pkg/results"
	"github.com/rotkonetworks/intspeed/pkg/speedtest"
	"github.com/rotkonetworks/intspeed/pkg/web"
	"github.com/spf13/cobra"
)

const version = "2.0.0"

var (
	outputDir     string
	threads       int
	timeout       int
	verbose       bool
	generateHTML  bool
)

func main() {
	var rootCmd = &cobra.Command{
		Use:     "intspeed",
		Short:   "International network performance testing",
		Version: version,
	}

	rootCmd.PersistentFlags().StringVarP(&outputDir, "output", "o", "results", "Output directory")
	rootCmd.PersistentFlags().IntVarP(&threads, "threads", "t", 2, "Threads per test")
	rootCmd.PersistentFlags().IntVar(&timeout, "timeout", 180, "Timeout seconds per location")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	var testCmd = &cobra.Command{
		Use:   "test",
		Short: "Run speed tests",
		Run:   runTest,
	}
	testCmd.Flags().BoolVar(&generateHTML, "html", false, "Generate HTML report")

	var locationsCmd = &cobra.Command{
		Use:   "locations",
		Short: "List test locations",
		Run:   listLocations,
	}

	var htmlCmd = &cobra.Command{
		Use:   "html [results.json]",
		Short: "Generate HTML from results",
		Args:  cobra.MaximumNArgs(1),
		Run:   generateHTMLReport,
	}

	rootCmd.AddCommand(testCmd, locationsCmd, htmlCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func runTest(cmd *cobra.Command, args []string) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("create output dir: %v", err)
	}

	fmt.Printf("🌍 intspeed v%s from rotko.net\n", version)
	fmt.Printf("📍 Testing %d locations sequentially for accurate results\n", len(locations.GlobalLocations))
	fmt.Printf("⏱️  Estimated time: %d minutes\n", (len(locations.GlobalLocations)*timeout)/60)

	client := speedtest.NewClient(speedtest.Config{
		Threads: threads,
		Timeout: time.Duration(timeout) * time.Second,
	})

	userInfo, err := client.GetUserInfo(context.Background())
	if err != nil {
		log.Printf("warning: get user info: %v", err)
	} else if verbose {
		fmt.Printf("📡 Your IP: %s (%s) [%s, %s]\n", userInfo.IP, userInfo.Isp, userInfo.Lat, userInfo.Lon)
	}

	testResults := &results.TestResults{
		UserInfo:  userInfo,
		Tests:     make([]speedtest.Result, 0, len(locations.GlobalLocations)),
		Timestamp: time.Now(),
		Version:   version,
	}

	fmt.Println()

	// Test each location sequentially
	for i, location := range locations.GlobalLocations {
		fmt.Printf("🔍 [%d/%d] Testing %s...\n", i+1, len(locations.GlobalLocations), location.Name)
		
		result := client.TestLocation(context.Background(), location)
		testResults.Tests = append(testResults.Tests, result)

		if result.Success {
			best := result.BestISP
			stats := result.AggregatedStats
			fmt.Printf("✅ [%d/%d] %s (%d/%d ISPs): Best: %s %.1fms ↓%.1f/↑%.1f | Avg: %.1fms ↓%.1f/↑%.1f Mbps\n",
				i+1, len(locations.GlobalLocations), location.Name,
				stats.SuccessfulISPs, stats.TotalISPs,
				best.ISP, best.Latency, best.DownloadSpeed, best.UploadSpeed,
				stats.AvgLatency, stats.AvgDownload, stats.AvgUpload)
		} else {
			fmt.Printf("❌ [%d/%d] %s: %s (%d ISPs tried)\n",
				i+1, len(locations.GlobalLocations), location.Name, result.Error, result.ServersAttempted)
		}

		// Brief pause between tests to be nice to servers
		if i < len(locations.GlobalLocations)-1 {
			time.Sleep(2 * time.Second)
		}
	}

	testResults.SortByLatency()

	// Save results
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := filepath.Join(outputDir, fmt.Sprintf("results_%s.json", timestamp))
	
	if err := testResults.Save(filename); err != nil {
		log.Fatalf("save results: %v", err)
	}

	// Save as latest
	latestFile := filepath.Join(outputDir, "latest.json")
	testResults.Save(latestFile)

	fmt.Printf("\n📊 Results saved: %s\n", filename)

	// Show stats
	stats := testResults.GetStats()
	fmt.Printf("✅ Success: %d/%d (%.1f%%)\n", stats.Successful, stats.Total, stats.SuccessRate)
	if stats.Successful > 0 {
		fmt.Printf("📊 Avg: %.1fms, ↓%.1f Mbps, ↑%.1f Mbps\n",
			stats.AvgLatency, stats.AvgDownload, stats.AvgUpload)
		
		if stats.BestLatency != nil {
			fmt.Printf("🏆 Best latency: %s → %s (%.1fms)\n",
				stats.BestLatency.Location.Name, stats.BestLatency.ServerCity(), stats.BestLatency.Latency())
		}
		if stats.BestDownload != nil {
			fmt.Printf("🏆 Best download: %s → %s (%.1f Mbps)\n",
				stats.BestDownload.Location.Name, stats.BestDownload.ServerCity(), stats.BestDownload.DownloadSpeed())
		}
	}

	if generateHTML {
		htmlFile := filepath.Join(outputDir, fmt.Sprintf("report_%s.html", timestamp))
		html := web.GenerateHTML(testResults)
		if err := os.WriteFile(htmlFile, []byte(html), 0644); err != nil {
			log.Printf("save HTML: %v", err)
		} else {
			fmt.Printf("📄 HTML report: %s\n", htmlFile)
		}
	}
}

func listLocations(cmd *cobra.Command, args []string) {
	fmt.Printf("🌍 Global Test Locations (%d total)\n\n", len(locations.GlobalLocations))
	
	regions := locations.GetByRegion()
	for _, region := range locations.GetRegions() {
		fmt.Printf("📍 %s:\n", region)
		for _, loc := range regions[region] {
			fmt.Printf("   • %s (%s) - %s\n", loc.Name, loc.CountryCode, loc.Description)
		}
		fmt.Println()
	}
}

func generateHTMLReport(cmd *cobra.Command, args []string) {
	var filename string
	if len(args) > 0 {
		filename = args[0]
	} else {
		filename = filepath.Join(outputDir, "latest.json")
	}

	testResults, err := results.Load(filename)
	if err != nil {
		log.Fatalf("load results: %v", err)
	}

	html := web.GenerateHTML(testResults)
	htmlFile := strings.TrimSuffix(filename, ".json") + ".html"
	
	if err := os.WriteFile(htmlFile, []byte(html), 0644); err != nil {
		log.Fatalf("save HTML: %v", err)
	}

	fmt.Printf("📄 HTML report generated: %s\n", htmlFile)
}
