package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	DownloadSpeed prometheus.Gauge
	UploadSpeed   prometheus.Gauge
	Ping          prometheus.Gauge
}

func newMetrics() *Metrics {
	return &Metrics{
		DownloadSpeed: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "speedtest_download_speed_mbps",
			Help: "Download speed in Mbps",
		}),
		UploadSpeed: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "speedtest_upload_speed_mbps",
			Help: "Upload speed in Mbps",
		}),
		Ping: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "speedtest_ping_ms",
			Help: "Ping in milliseconds",
		}),
	}
}

func (m *Metrics) register() {
	prometheus.MustRegister(m.DownloadSpeed, m.UploadSpeed, m.Ping)
}

type SpeedTestResult struct {
	Ping     float64 `json:"ping"`
	Download float64 `json:"download"`
	Upload   float64 `json:"upload"`
}

type AggregationMode string

const (
	AggregationBest    AggregationMode = "best"
	AggregationAverage AggregationMode = "average"
	AggregationMedian  AggregationMode = "median"
	AggregationWorst   AggregationMode = "worst"
)

func parseAggregationMode(s string) (AggregationMode, error) {
	switch mode := AggregationMode(strings.ToLower(strings.TrimSpace(s))); mode {
	case AggregationBest, AggregationAverage, AggregationMedian, AggregationWorst:
		return mode, nil
	case "avg":
		return AggregationAverage, nil
	default:
		return "", fmt.Errorf("unsupported aggregation mode %q", s)
	}
}

func parseServers(s string) []string {
	var servers []string
	for _, server := range strings.Split(s, ",") {
		if id := strings.TrimSpace(server); id != "" {
			servers = append(servers, id)
		}
	}
	return servers
}

var serverIDRegex = regexp.MustCompile(`^\s*(\d+)\)`)

// speedtest --list からサーバIDの一覧を取得する
func getServerList() []string {
	cmd := exec.Command("speedtest", "--list")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		log.Printf("Failed to get server list: %v", err)
		return nil
	}

	var ids []string
	for _, line := range strings.Split(out.String(), "\n") {
		if m := serverIDRegex.FindStringSubmatch(line); m != nil {
			ids = append(ids, m[1])
		}
	}
	return ids
}

func runSpeedtest(serverID string) (*SpeedTestResult, error) {
	if serverID != "" {
		return runSpeedtestByID(serverID)
	}

	args := []string{"--json", "--secure"}

	cmd := exec.Command("speedtest", args...)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Printf("Speedtest failed (server=%q): %v\nStderr: %s", serverID, err, stderr.String())
		return nil, err
	}

	log.Printf("Speedtest output (server=%q): %s", serverID, out.String())

	var result SpeedTestResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		log.Printf("JSON parse error: %v", err)
		return nil, err
	}

	return &result, nil
}

func runSpeedtestByID(serverID string) (*SpeedTestResult, error) {
	cmd := exec.Command("python3", "/app/speedtest_by_id.py", serverID)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Printf("Speedtest failed (server=%q): %v\nStderr: %s", serverID, err, stderr.String())
		return nil, err
	}

	log.Printf("Speedtest output (server=%q): %s", serverID, out.String())

	var result SpeedTestResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		log.Printf("JSON parse error: %v", err)
		return nil, err
	}

	return &result, nil
}

func isValidResult(r *SpeedTestResult) bool {
	return r.Download > 0 && r.Upload > 0
}

func aggregateResults(results []*SpeedTestResult, mode AggregationMode) *SpeedTestResult {
	if len(results) == 0 {
		return nil
	}

	switch mode {
	case AggregationAverage:
		return averageResults(results)
	case AggregationMedian:
		return medianResults(results)
	case AggregationWorst:
		return worstResult(results)
	case AggregationBest:
		fallthrough
	default:
		return bestResult(results)
	}
}

func bestResult(results []*SpeedTestResult) *SpeedTestResult {
	agg := *results[0]
	for _, result := range results[1:] {
		if result.Download > agg.Download {
			agg.Download = result.Download
		}
		if result.Upload > agg.Upload {
			agg.Upload = result.Upload
		}
		if result.Ping < agg.Ping {
			agg.Ping = result.Ping
		}
	}
	return &agg
}

func worstResult(results []*SpeedTestResult) *SpeedTestResult {
	agg := *results[0]
	for _, result := range results[1:] {
		if result.Download < agg.Download {
			agg.Download = result.Download
		}
		if result.Upload < agg.Upload {
			agg.Upload = result.Upload
		}
		if result.Ping > agg.Ping {
			agg.Ping = result.Ping
		}
	}
	return &agg
}

func averageResults(results []*SpeedTestResult) *SpeedTestResult {
	agg := &SpeedTestResult{}
	for _, result := range results {
		agg.Download += result.Download
		agg.Upload += result.Upload
		agg.Ping += result.Ping
	}
	count := float64(len(results))
	agg.Download /= count
	agg.Upload /= count
	agg.Ping /= count
	return agg
}

func medianResults(results []*SpeedTestResult) *SpeedTestResult {
	downloads := make([]float64, 0, len(results))
	uploads := make([]float64, 0, len(results))
	pings := make([]float64, 0, len(results))
	for _, result := range results {
		downloads = append(downloads, result.Download)
		uploads = append(uploads, result.Upload)
		pings = append(pings, result.Ping)
	}
	return &SpeedTestResult{
		Download: median(downloads),
		Upload:   median(uploads),
		Ping:     median(pings),
	}
}

func median(values []float64) float64 {
	sort.Float64s(values)
	middle := len(values) / 2
	if len(values)%2 == 1 {
		return values[middle]
	}
	return (values[middle-1] + values[middle]) / 2
}

func (m *Metrics) set(r *SpeedTestResult) {
	m.DownloadSpeed.Set(r.Download / (1024 * 1024))
	m.UploadSpeed.Set(r.Upload / (1024 * 1024))
	m.Ping.Set(r.Ping)
}

const maxFallbackServers = 5

// servers が空の場合はデフォルト+動的リストで試行、指定がある場合は全サーバを計測して集計する
func collectMetrics(m *Metrics, servers []string, aggregation AggregationMode) {
	serversToTest := append([]string(nil), servers...)
	if len(servers) == 0 {
		// デフォルトサーバで試行
		if result, err := runSpeedtest(""); err == nil && isValidResult(result) {
			m.set(result)
			return
		}
		log.Printf("Default server failed or returned zero values, fetching server list...")

		serversToTest = getServerList()
		if len(serversToTest) > maxFallbackServers {
			serversToTest = serversToTest[:maxFallbackServers]
		}
	}

	var results []*SpeedTestResult
	for _, id := range serversToTest {
		result, err := runSpeedtest(id)
		if err == nil && isValidResult(result) {
			log.Printf("Got valid metrics from server %s", id)
			results = append(results, result)
			continue
		}
		log.Printf("Server %s failed or returned zero values", id)
	}

	if len(results) == 0 {
		log.Printf("All servers failed, metrics not updated")
		return
	}

	result := aggregateResults(results, aggregation)
	log.Printf(
		"Using %s aggregate from %d/%d successful servers: download=%.2f Mbps upload=%.2f Mbps ping=%.2f ms",
		aggregation,
		len(results),
		len(serversToTest),
		result.Download/(1024*1024),
		result.Upload/(1024*1024),
		result.Ping,
	)
	m.set(result)
}

func main() {
	var serversFlag string
	var aggregationFlag string
	flag.StringVar(&serversFlag, "servers", "", "comma-separated list of speedtest server IDs to use (e.g. 1234,5678)")
	flag.StringVar(&aggregationFlag, "aggregation", string(AggregationBest), "how to aggregate multiple server results: best, average, median, or worst")
	flag.Parse()

	servers := parseServers(serversFlag)
	aggregation, err := parseAggregationMode(aggregationFlag)
	if err != nil {
		log.Fatal(err)
	}

	metrics := newMetrics()
	metrics.register()

	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		collectMetrics(metrics, servers, aggregation)
		promhttp.Handler().ServeHTTP(w, r)
	})

	log.Printf("Starting speedtest exporter server on :8080 (servers=%q aggregation=%s)", servers, aggregation)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
