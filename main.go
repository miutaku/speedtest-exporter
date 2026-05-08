package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os/exec"
	"regexp"
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

// serverID が空文字の場合はデフォルトサーバを使用する
func runSpeedtest(serverID string) (*SpeedTestResult, error) {
	args := []string{"--json", "--secure"}
	if serverID != "" {
		args = append(args, "--server", serverID)
	}

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

func isValidResult(r *SpeedTestResult) bool {
	return r.Download > 0 && r.Upload > 0
}

func (m *Metrics) set(r *SpeedTestResult) {
	m.DownloadSpeed.Set(r.Download / (1024 * 1024))
	m.UploadSpeed.Set(r.Upload / (1024 * 1024))
	m.Ping.Set(r.Ping)
}

const maxFallbackServers = 5

// servers が空の場合はデフォルト+動的リストで試行、指定がある場合はその順で試行
func collectMetrics(m *Metrics, servers []string) {
	if len(servers) == 0 {
		// デフォルトサーバで試行
		if result, err := runSpeedtest(""); err == nil && isValidResult(result) {
			m.set(result)
			return
		}
		log.Printf("Default server failed or returned zero values, fetching server list...")

		servers = getServerList()
		if len(servers) > maxFallbackServers {
			servers = servers[:maxFallbackServers]
		}
	}

	for _, id := range servers {
		result, err := runSpeedtest(id)
		if err == nil && isValidResult(result) {
			log.Printf("Got valid metrics from server %s", id)
			m.set(result)
			return
		}
		log.Printf("Server %s failed or returned zero values", id)
	}

	log.Printf("All servers failed, metrics not updated")
}

func main() {
	var serversFlag string
	flag.StringVar(&serversFlag, "servers", "", "comma-separated list of speedtest server IDs to use (e.g. 1234,5678)")
	flag.Parse()

	var servers []string
	for _, s := range strings.Split(serversFlag, ",") {
		if id := strings.TrimSpace(s); id != "" {
			servers = append(servers, id)
		}
	}

	metrics := newMetrics()
	metrics.register()

	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		collectMetrics(metrics, servers)
		promhttp.Handler().ServeHTTP(w, r)
	})

	log.Println("Starting speedtest exporter server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
