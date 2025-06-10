package speedtest

import (
   "context"
   "encoding/json"
   "fmt"
   "io"
   "math"
   "math/rand"
   "net/http"
   "net/url"
   "sort"
   "time"

   "github.com/rotkonetworks/intspeed/pkg/locations"
   "github.com/showwin/speedtest-go/speedtest"
)

type ISPResult struct {
   ServerID      string  `json:"server_id"`
   ServerName    string  `json:"server_name"`
   ISP           string  `json:"isp"`
   Distance      float64 `json:"distance_km"`
   Latency       float64 `json:"latency_ms"`
   Jitter        float64 `json:"jitter_ms"`
   DownloadSpeed float64 `json:"download_mbps"`
   UploadSpeed   float64 `json:"upload_mbps"`
   PacketLoss    float64 `json:"packet_loss_percent"`
   Success       bool    `json:"success"`
   Error         string  `json:"error,omitempty"`
}

type Result struct {
   Location         locations.Location `json:"location"`
   ISPResults       []ISPResult        `json:"isp_results"`
   BestISP          *ISPResult         `json:"best_isp"`
   AggregatedStats  AggregatedStats    `json:"aggregated_stats"`
   Timestamp        time.Time          `json:"timestamp"`
   Success          bool               `json:"success"`
   Error            string             `json:"error,omitempty"`
   ServersAttempted int                `json:"servers_attempted"`
   TestDuration     float64            `json:"test_duration_seconds"`
}

// Compatibility methods for existing results package
func (r *Result) Latency() float64 {
   if r.BestISP != nil {
   	return r.BestISP.Latency
   }
   return r.AggregatedStats.AvgLatency
}

func (r *Result) DownloadSpeed() float64 {
   if r.BestISP != nil {
   	return r.BestISP.DownloadSpeed
   }
   return r.AggregatedStats.AvgDownload
}

func (r *Result) UploadSpeed() float64 {
   if r.BestISP != nil {
   	return r.BestISP.UploadSpeed
   }
   return r.AggregatedStats.AvgUpload
}

func (r *Result) ServerCity() string {
   if r.BestISP != nil {
   	return r.BestISP.ServerName
   }
   return r.Location.Name
}

type AggregatedStats struct {
   AvgLatency       float64 `json:"avg_latency_ms"`
   MinLatency       float64 `json:"min_latency_ms"`
   MaxLatency       float64 `json:"max_latency_ms"`
   AvgDownload      float64 `json:"avg_download_mbps"`
   MaxDownload      float64 `json:"max_download_mbps"`
   AvgUpload        float64 `json:"avg_upload_mbps"`
   MaxUpload        float64 `json:"max_upload_mbps"`
   SuccessfulISPs   int     `json:"successful_isps"`
   TotalISPs        int     `json:"total_isps"`
   SuccessRate      float64 `json:"success_rate_percent"`
}

type Config struct {
   Threads      int
   Timeout      time.Duration
   UserAgent    string
   MaxISPs      int
}

type Client struct {
   cfg    Config
   client *speedtest.Speedtest
}

type SpeedtestServer struct {
   ID       string  `json:"id"`
   Name     string  `json:"name"`
   Country  string  `json:"country"`
   CC       string  `json:"cc"`
   Sponsor  string  `json:"sponsor"`
   Host     string  `json:"host"`
   URL      string  `json:"url"`
   Lat      string  `json:"lat"`
   Lon      string  `json:"lon"`
   Distance float64 `json:"distance"`
}

func NewClient(cfg Config) *Client {
   if cfg.UserAgent == "" {
   	cfg.UserAgent = "intspeed/2.0"
   }
   if cfg.Timeout == 0 {
   	cfg.Timeout = 180 * time.Second
   }
   if cfg.Threads == 0 {
   	cfg.Threads = 2
   }
   if cfg.MaxISPs == 0 {
   	cfg.MaxISPs = 5
   }

   client := speedtest.New(speedtest.WithUserConfig(&speedtest.UserConfig{
   	UserAgent: cfg.UserAgent,
   }))

   return &Client{
   	cfg:    cfg,
   	client: client,
   }
}

func (c *Client) TestLocation(ctx context.Context, location locations.Location) Result {
   startTime := time.Now()
   result := Result{
   	Location:  location,
   	Timestamp: startTime,
   }

   ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
   defer cancel()

   // Find servers for this city
   servers, err := c.fetchServersForCity(ctx, location.Name)
   if err != nil {
   	result.Error = fmt.Sprintf("API error: %v", err)
   	result.TestDuration = time.Since(startTime).Seconds()
   	return result
   }

   if len(servers) == 0 {
   	result.Error = fmt.Sprintf("no servers found for %s", location.Name)
   	result.TestDuration = time.Since(startTime).Seconds()
   	return result
   }

   // Select random unique ISPs
   selectedServers := c.selectRandomISPs(servers, c.cfg.MaxISPs)
   result.ServersAttempted = len(selectedServers)

   fmt.Printf("    Found %d ISPs for %s, testing %d:\n", len(servers), location.Name, len(selectedServers))
   for i, server := range selectedServers {
   	fmt.Printf("      %d. %s - %s (%.0fkm)\n", i+1, server.Sponsor, server.Name, server.Distance)
   }

   // Test selected ISPs
   result.ISPResults = c.testMultipleISPs(ctx, selectedServers)
   result.AggregatedStats = c.calculateAggregatedStats(result.ISPResults)
   result.BestISP = c.findBestISP(result.ISPResults)
   result.Success = result.AggregatedStats.SuccessfulISPs > 0

   if !result.Success {
   	result.Error = "all ISPs failed"
   }

   result.TestDuration = time.Since(startTime).Seconds()
   return result
}

func (c *Client) fetchServersForCity(ctx context.Context, cityName string) ([]SpeedtestServer, error) {
   apiURL := fmt.Sprintf("https://www.speedtest.net/api/js/servers?engine=js&https_functional=true&limit=100&search=%s", 
   	url.QueryEscape(cityName))
   
   req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
   if err != nil {
   	return nil, err
   }
   
   req.Header.Set("User-Agent", c.cfg.UserAgent)
   req.Header.Set("Accept", "application/json")
   
   resp, err := http.DefaultClient.Do(req)
   if err != nil {
   	return nil, err
   }
   defer resp.Body.Close()
   
   if resp.StatusCode != 200 {
   	return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
   }
   
   body, err := io.ReadAll(resp.Body)
   if err != nil {
   	return nil, err
   }
   
   var servers []SpeedtestServer
   if err := json.Unmarshal(body, &servers); err != nil {
   	return nil, err
   }
   
   return servers, nil
}

func (c *Client) selectRandomISPs(servers []SpeedtestServer, maxISPs int) []SpeedtestServer {
   // Group by unique ISP (sponsor)
   ispMap := make(map[string][]SpeedtestServer)
   for _, server := range servers {
   	ispMap[server.Sponsor] = append(ispMap[server.Sponsor], server)
   }

   // Get list of unique ISPs
   var isps []string
   for isp := range ispMap {
   	isps = append(isps, isp)
   }

   // Shuffle ISPs
   rand.Shuffle(len(isps), func(i, j int) {
   	isps[i], isps[j] = isps[j], isps[i]
   })

   // Select up to maxISPs, pick closest server from each ISP
   var selected []SpeedtestServer
   for i, isp := range isps {
   	if i >= maxISPs {
   		break
   	}
   	
   	// Sort servers for this ISP by distance and pick the closest
   	ispServers := ispMap[isp]
   	sort.Slice(ispServers, func(i, j int) bool {
   		return ispServers[i].Distance < ispServers[j].Distance
   	})
   	
   	selected = append(selected, ispServers[0])
   }

   return selected
}

func (c *Client) testMultipleISPs(ctx context.Context, servers []SpeedtestServer) []ISPResult {
   var results []ISPResult

   // Test ISPs sequentially to avoid overwhelming servers
   for i, server := range servers {
   	fmt.Printf("    [%d/%d] Testing %s - %s...\n", i+1, len(servers), server.Sponsor, server.Name)
   	
   	ispResult := c.testSingleISP(ctx, server)
   	results = append(results, ispResult)

   	status := "✅"
   	if !ispResult.Success {
   		status = "❌"
   	}
   	
   	if ispResult.Success {
   		fmt.Printf("    %s %s: %.1fms, ↓%.1f Mbps, ↑%.1f Mbps\n", 
   			status, server.Sponsor, ispResult.Latency, ispResult.DownloadSpeed, ispResult.UploadSpeed)
   	} else {
   		fmt.Printf("    %s %s: %s\n", status, server.Sponsor, ispResult.Error)
   	}

   	// Brief pause between tests
   	if i < len(servers)-1 {
   		time.Sleep(1 * time.Second)
   	}
   }

   return results
}

func (c *Client) testSingleISP(ctx context.Context, apiServer SpeedtestServer) ISPResult {
   result := ISPResult{
   	ServerID:   apiServer.ID,
   	ServerName: apiServer.Name,
   	ISP:        apiServer.Sponsor,
   	Distance:   apiServer.Distance,
   }

   // Convert to speedtest-go server format
   server := &speedtest.Server{
   	ID:       apiServer.ID,
   	Name:     apiServer.Name,
   	Country:  apiServer.Country,
   	Sponsor:  apiServer.Sponsor,
   	URL:      apiServer.URL,
   	Host:     apiServer.Host,
   	Lat:      apiServer.Lat,
   	Lon:      apiServer.Lon,
   	Distance: apiServer.Distance,
   	Context:  c.client,
   }

   server.Context.SetNThread(c.cfg.Threads)

   // Individual test timeout
   testCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
   defer cancel()

   // 1. Ping test
   if err := server.PingTestContext(testCtx, nil); err != nil {
   	result.Error = fmt.Sprintf("ping failed: %v", err)
   	return result
   }

   result.Latency = float64(server.Latency.Nanoseconds()) / 1e6
   result.Jitter = float64(server.Jitter.Nanoseconds()) / 1e6

   if result.Latency <= 0 || result.Latency > 5000 { // Sanity check
   	result.Error = "invalid latency"
   	return result
   }

   // 2. Download test
   if err := server.DownloadTestContext(testCtx); err != nil {
   	result.Error = fmt.Sprintf("download failed: %v", err)
   	return result
   }

   dlSpeed := server.DLSpeed.Mbps()
   if math.IsNaN(dlSpeed) || math.IsInf(dlSpeed, 0) || dlSpeed < 0 {
   	dlSpeed = 0
   }
   result.DownloadSpeed = dlSpeed

   // 3. Upload test
   if err := server.UploadTestContext(testCtx); err != nil {
   	result.Error = fmt.Sprintf("upload failed: %v", err)
   	return result
   }

   ulSpeed := server.ULSpeed.Mbps()
   if math.IsNaN(ulSpeed) || math.IsInf(ulSpeed, 0) || ulSpeed < 0 {
   	ulSpeed = 0
   }
   result.UploadSpeed = ulSpeed

   // Packet loss (if available)
   if server.PacketLoss.Sent > 0 {
   	loss := server.PacketLoss.LossPercent()
   	if loss >= 0 && loss <= 100 {
   		result.PacketLoss = loss
   	}
   }

   // Consider test successful if we got reasonable speeds
   if result.DownloadSpeed >= 0.5 || result.UploadSpeed >= 0.5 {
   	result.Success = true
   } else {
   	result.Error = fmt.Sprintf("speeds too low: dl=%.1f ul=%.1f", result.DownloadSpeed, result.UploadSpeed)
   }

   return result
}

func (c *Client) calculateAggregatedStats(results []ISPResult) AggregatedStats {
   stats := AggregatedStats{
   	TotalISPs:  len(results),
   	MinLatency: math.MaxFloat64,
   }

   if len(results) == 0 {
   	return stats
   }

   var totalLatency, totalDownload, totalUpload float64
   
   for _, result := range results {
   	if !result.Success {
   		continue
   	}

   	stats.SuccessfulISPs++
   	totalLatency += result.Latency
   	totalDownload += result.DownloadSpeed
   	totalUpload += result.UploadSpeed

   	if result.Latency < stats.MinLatency {
   		stats.MinLatency = result.Latency
   	}
   	if result.Latency > stats.MaxLatency {
   		stats.MaxLatency = result.Latency
   	}
   	if result.DownloadSpeed > stats.MaxDownload {
   		stats.MaxDownload = result.DownloadSpeed
   	}
   	if result.UploadSpeed > stats.MaxUpload {
   		stats.MaxUpload = result.UploadSpeed
   	}
   }

   if stats.SuccessfulISPs > 0 {
   	stats.AvgLatency = totalLatency / float64(stats.SuccessfulISPs)
   	stats.AvgDownload = totalDownload / float64(stats.SuccessfulISPs)
   	stats.AvgUpload = totalUpload / float64(stats.SuccessfulISPs)
   	stats.SuccessRate = float64(stats.SuccessfulISPs) / float64(stats.TotalISPs) * 100
   }

   if stats.MinLatency == math.MaxFloat64 {
   	stats.MinLatency = 0
   }

   return stats
}

func (c *Client) findBestISP(results []ISPResult) *ISPResult {
   var best *ISPResult
   bestScore := -1.0

   for i, result := range results {
   	if !result.Success {
   		continue
   	}

   	// Scoring: prioritize low latency and high bandwidth
   	latencyScore := 1000.0 / (result.Latency + 1)
   	bandwidthScore := result.DownloadSpeed + result.UploadSpeed
   	score := latencyScore + (bandwidthScore * 2)

   	if score > bestScore {
   		bestScore = score
   		best = &results[i]
   	}
   }

   return best
}

func (c *Client) GetUserInfo(ctx context.Context) (*speedtest.User, error) {
   return c.client.FetchUserInfoContext(ctx)
}
