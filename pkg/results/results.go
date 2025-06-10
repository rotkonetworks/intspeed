package results

import (
	"encoding/json"
	"os"
	"sort"
	"time"

	"github.com/rotkonetworks/intspeed/pkg/speedtest"
	extspeedtest "github.com/showwin/speedtest-go/speedtest"
)

type TestResults struct {
	UserInfo  *extspeedtest.User      `json:"user_info"`
	Tests     []speedtest.Result      `json:"tests"`
	Timestamp time.Time               `json:"timestamp"`
	Version   string                  `json:"version"`
}

type Stats struct {
	Total         int                 `json:"total"`
	Successful    int                 `json:"successful"`
	SuccessRate   float64             `json:"success_rate"`
	AvgLatency    float64             `json:"avg_latency"`
	AvgDownload   float64             `json:"avg_download"`
	AvgUpload     float64             `json:"avg_upload"`
	BestLatency   *speedtest.Result   `json:"best_latency"`
	BestDownload  *speedtest.Result   `json:"best_download"`
}

func (r *TestResults) SortByLatency() {
	sort.Slice(r.Tests, func(i, j int) bool {
		if !r.Tests[i].Success && !r.Tests[j].Success {
			return false
		}
		if !r.Tests[i].Success {
			return false
		}
		if !r.Tests[j].Success {
			return true
		}
		return r.Tests[i].Latency() < r.Tests[j].Latency()
	})
}

func (r *TestResults) Save(filename string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

func (r *TestResults) GetStats() Stats {
	stats := Stats{
		Total: len(r.Tests),
	}

	if len(r.Tests) == 0 {
		return stats
	}

	var totalLatency, totalDownload, totalUpload float64
	var bestLatency, bestDownload *speedtest.Result

	for i, test := range r.Tests {
		if !test.Success {
			continue
		}

		stats.Successful++
		totalLatency += test.Latency()
		totalDownload += test.DownloadSpeed()
		totalUpload += test.UploadSpeed()

		if bestLatency == nil || test.Latency() < bestLatency.Latency() {
			bestLatency = &r.Tests[i]
		}

		if bestDownload == nil || test.DownloadSpeed() > bestDownload.DownloadSpeed() {
			bestDownload = &r.Tests[i]
		}
	}

	if stats.Successful > 0 {
		stats.SuccessRate = float64(stats.Successful) / float64(stats.Total) * 100
		stats.AvgLatency = totalLatency / float64(stats.Successful)
		stats.AvgDownload = totalDownload / float64(stats.Successful)
		stats.AvgUpload = totalUpload / float64(stats.Successful)
		stats.BestLatency = bestLatency
		stats.BestDownload = bestDownload
	}

	return stats
}

func Load(filename string) (*TestResults, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var results TestResults
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}

	return &results, nil
}
