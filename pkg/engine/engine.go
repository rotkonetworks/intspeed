// Package engine is the portable speed-test engine shared by the native CLI
// and the browser WASM build. It only uses net/http (mapped to fetch under
// js/wasm), so every measurement works identically on both targets against
// the endpoints registry.
package engine

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/rotkonetworks/intspeed/pkg/endpoints"
)

// browserMode: under js/wasm, requests go through fetch, where non-safelisted
// headers (like Range) trigger a CORS preflight most test servers reject.
var browserMode = runtime.GOOS == "js"

type Options struct {
	DownloadBytes int64
	UploadBytes   int64
	PingCount     int
	BrowserOnly   bool          // restrict to CORS-open endpoints (set by the wasm build)
	OpTimeout     time.Duration // timeout per single measurement
	Locations     []string      // subset filter; empty = all
}

func (o *Options) defaults() {
	if o.DownloadBytes == 0 {
		o.DownloadBytes = 15_000_000
	}
	if o.UploadBytes == 0 {
		o.UploadBytes = 6_000_000
	}
	if o.PingCount == 0 {
		o.PingCount = 4
	}
	if o.OpTimeout == 0 {
		o.OpTimeout = 45 * time.Second
	}
}

type Progress struct {
	Type     string  `json:"type"` // location_start | latency | download | upload | location_done | sweep_done
	Location string  `json:"location"`
	Endpoint string  `json:"endpoint,omitempty"`
	Value    float64 `json:"value,omitempty"` // ms or Mbps depending on Type
	Index    int     `json:"index"`
	Total    int     `json:"total"`
	Error    string  `json:"error,omitempty"`
}

type EndpointResult struct {
	Name      string  `json:"name"`
	Kind      string  `json:"kind"`
	LatencyMs float64 `json:"latency_ms"`
	JitterMs  float64 `json:"jitter_ms"`
	Error     string  `json:"error,omitempty"`
}

type LocationResult struct {
	Location     string           `json:"location"`
	LatencyMs    float64          `json:"latency_ms"`
	JitterMs     float64          `json:"jitter_ms"`
	DownloadMbps float64          `json:"download_mbps"`
	UploadMbps   float64          `json:"upload_mbps"`
	PingVia      string           `json:"ping_via,omitempty"`
	DownloadVia  string           `json:"download_via,omitempty"`
	UploadVia    string           `json:"upload_via,omitempty"`
	Endpoints    []EndpointResult `json:"endpoints"`
	Error        string           `json:"error,omitempty"`
}

var client = &http.Client{}

// Sweep tests every registry location in order, invoking cb (if non-nil)
// as measurements complete.
func Sweep(ctx context.Context, reg *endpoints.Registry, opts Options, cb func(Progress)) []LocationResult {
	opts.defaults()
	emit := func(p Progress) {
		if cb != nil {
			cb(p)
		}
	}

	var locs []endpoints.LocationEndpoints
	for _, l := range reg.Locations {
		if len(opts.Locations) > 0 && !containsFold(opts.Locations, l.Name) {
			continue
		}
		locs = append(locs, l)
	}

	results := make([]LocationResult, 0, len(locs))
	for i, loc := range locs {
		if ctx.Err() != nil {
			break
		}
		emit(Progress{Type: "location_start", Location: loc.Name, Index: i + 1, Total: len(locs)})
		res := testLocation(ctx, loc, opts, emit)
		results = append(results, res)
		emit(Progress{Type: "location_done", Location: loc.Name, Index: i + 1, Total: len(locs), Error: res.Error})
	}
	emit(Progress{Type: "sweep_done", Total: len(locs)})
	return results
}

func testLocation(ctx context.Context, loc endpoints.LocationEndpoints, opts Options, emit func(Progress)) LocationResult {
	res := LocationResult{Location: loc.Name}

	eps := loc.Endpoints
	if opts.BrowserOnly {
		eps = loc.Browser()
	}
	if len(eps) == 0 {
		res.Error = "no usable endpoints"
		return res
	}

	// Latency on every endpoint; the fastest ones win the throughput tests.
	type measured struct {
		ep   endpoints.Endpoint
		base string
		lat  float64
	}
	var ok []measured
	for _, ep := range eps {
		lat, jit, base, err := measureLatency(ctx, ep, opts)
		er := EndpointResult{Name: ep.Name, Kind: ep.Kind, LatencyMs: lat, JitterMs: jit}
		if err != nil {
			er.Error = err.Error()
			res.Endpoints = append(res.Endpoints, er)
			continue
		}
		res.Endpoints = append(res.Endpoints, er)
		ok = append(ok, measured{ep, base, lat})
		emit(Progress{Type: "latency", Location: loc.Name, Endpoint: ep.Name, Value: lat})
		if lat < res.LatencyMs || res.LatencyMs == 0 {
			res.LatencyMs, res.JitterMs, res.PingVia = lat, jit, ep.Name
		}
	}
	if len(ok) == 0 {
		res.Error = "all endpoints unreachable"
		return res
	}

	// Download from the lowest-latency endpoint.
	bestDL := ok[0]
	for _, m := range ok[1:] {
		if m.lat < bestDL.lat {
			bestDL = m
		}
	}
	if mbps, err := measureDownload(ctx, bestDL.ep, bestDL.base, opts); err == nil {
		res.DownloadMbps, res.DownloadVia = mbps, bestDL.ep.Name
		emit(Progress{Type: "download", Location: loc.Name, Endpoint: bestDL.ep.Name, Value: mbps})
	} else {
		res.Error = fmt.Sprintf("download: %v", err)
	}

	// Upload to the lowest-latency upload-capable endpoint.
	var bestUL *measured
	for i := range ok {
		if ok[i].ep.Upload && (bestUL == nil || ok[i].lat < bestUL.lat) {
			bestUL = &ok[i]
		}
	}
	if bestUL != nil {
		if mbps, err := measureUpload(ctx, bestUL.ep, bestUL.base, opts); err == nil {
			res.UploadMbps, res.UploadVia = mbps, bestUL.ep.Name
			emit(Progress{Type: "upload", Location: loc.Name, Endpoint: bestUL.ep.Name, Value: mbps})
		} else if res.Error == "" {
			res.Error = fmt.Sprintf("upload: %v", err)
		}
	}
	return res
}

// measureLatency warms up the connection, then times PingCount small
// requests. Returns the minimum (closest to pure RTT), jitter as the mean
// absolute difference of consecutive samples, and the resolved base URL for
// librespeed endpoints (whose backend path prefix varies).
func measureLatency(ctx context.Context, ep endpoints.Endpoint, opts Options) (lat, jit float64, base string, err error) {
	var samples []float64
	found := false
	for _, b := range latencyURLs(ep) {
		var s []float64
		fail := false
		for i := 0; i <= opts.PingCount; i++ { // one extra: warm-up
			start := time.Now()
			if err = doSmallGet(ctx, ep, pingURL(ep, b), opts.OpTimeout); err != nil {
				fail = true
				break
			}
			if i > 0 {
				s = append(s, float64(time.Since(start).Nanoseconds())/1e6)
			}
		}
		// Accept a base only on a fully clean run; partial samples from a
		// flaky base must not leak into the result.
		if !fail {
			base, samples, found = b, s, true
			break
		}
	}
	if !found {
		if err == nil {
			err = fmt.Errorf("no latency samples")
		}
		return 0, 0, "", err
	}
	lat = math.MaxFloat64
	for _, s := range samples {
		lat = math.Min(lat, s)
	}
	for i := 1; i < len(samples); i++ {
		jit += math.Abs(samples[i] - samples[i-1])
	}
	if len(samples) > 1 {
		jit /= float64(len(samples) - 1)
	}
	return lat, jit, base, nil
}

// latencyURLs returns candidate base URLs. LibreSpeed installs differ on
// whether the backend lives at / or /backend, so both are tried.
func latencyURLs(ep endpoints.Endpoint) []string {
	switch ep.Kind {
	case "librespeed":
		b := strings.TrimSuffix(ep.URL, "/")
		if strings.HasSuffix(b, "/backend") {
			return []string{b, strings.TrimSuffix(b, "/backend")}
		}
		return []string{b, b + "/backend"}
	default:
		return []string{""}
	}
}

func pingURL(ep endpoints.Endpoint, base string) string {
	switch ep.Kind {
	case "ookla":
		return withParam("https://"+ep.Host+"/hi", "nocache")
	case "librespeed":
		return withParam(base+"/empty.php", "nocache")
	default: // file
		return withParam(ep.URL, "nocache")
	}
}

func doSmallGet(ctx context.Context, ep endpoints.Endpoint, url string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	if ep.Kind == "file" && !browserMode {
		// Range is not a CORS-safelisted header; in the browser we instead
		// read a few bytes of the full stream and abort via context cancel.
		req.Header.Set("Range", "bytes=0-0")
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	_, err = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	return err
}

func measureDownload(ctx context.Context, ep endpoints.Endpoint, base string, opts Options) (float64, error) {
	ctx, cancel := context.WithTimeout(ctx, opts.OpTimeout)
	defer cancel()

	var url string
	switch ep.Kind {
	case "ookla":
		url = withParam(fmt.Sprintf("https://%s/download?size=%d", ep.Host, opts.DownloadBytes), "nocache")
	case "librespeed":
		mb := (opts.DownloadBytes + 999_999) / 1_000_000
		url = withParam(fmt.Sprintf("%s/garbage.php?ckSize=%d", base, mb), "nocache")
	default:
		url = withParam(ep.URL, "nocache")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}
	if ep.Kind == "file" && !browserMode {
		req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", opts.DownloadBytes-1))
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("status %d", resp.StatusCode)
	}
	// Cap the read in case the server ignores Range and streams the full file.
	n, err := io.Copy(io.Discard, io.LimitReader(resp.Body, opts.DownloadBytes))
	secs := time.Since(start).Seconds()
	if n == 0 {
		if err == nil {
			err = fmt.Errorf("empty body")
		}
		return 0, err
	}
	return float64(n) * 8 / secs / 1e6, nil
}

func measureUpload(ctx context.Context, ep endpoints.Endpoint, base string, opts Options) (float64, error) {
	ctx, cancel := context.WithTimeout(ctx, opts.OpTimeout)
	defer cancel()

	var url string
	switch ep.Kind {
	case "ookla":
		url = withParam("https://"+ep.Host+"/upload", "nocache")
	case "librespeed":
		url = withParam(base+"/empty.php", "nocache")
	default:
		return 0, fmt.Errorf("endpoint has no upload")
	}

	payload := make([]byte, opts.UploadBytes)
	rand.Read(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return 0, err
	}
	// text/plain keeps this a CORS "simple request" (no OPTIONS preflight),
	// which most test servers don't implement.
	req.Header.Set("Content-Type", "text/plain")

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("status %d", resp.StatusCode)
	}
	return float64(opts.UploadBytes) * 8 / time.Since(start).Seconds() / 1e6, nil
}

func withParam(url, key string) string {
	sep := "?"
	if strings.Contains(url, "?") {
		sep = "&"
	}
	return fmt.Sprintf("%s%s%s=%d", url, sep, key, rand.Int63())
}

func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(strings.TrimSpace(v), s) {
			return true
		}
	}
	return false
}
