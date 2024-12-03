package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os/exec"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics struct to hold Prometheus metrics
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

// Speedtestコマンドを実行し、結果を取得
func runSpeedtest() (*SpeedTestResult, error) {
	cmd := exec.Command("speedtest", "--json", "--secure")
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		log.Printf("Speedtest command failed: %v\nStderr: %s", err, stderr.String())
		return nil, err
	}

	log.Printf("Speedtest raw output: %s", out.String())

	var result SpeedTestResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		return nil, err
	}

	return &result, nil
}

// メトリクスを収集する
func collectMetrics(m *Metrics) {
	result, err := runSpeedtest()
	if err != nil {
		log.Printf("Error running speedtest: %v", err)
		return
	}

	// Speedtestの結果をPrometheusメトリクスにセット
	m.DownloadSpeed.Set(result.Download / (1024 * 1024)) // bps to Mbps
	m.UploadSpeed.Set(result.Upload / (1024 * 1024))     // bps to Mbps
	m.Ping.Set(result.Ping)
}

func main() {
	metrics := newMetrics()
	metrics.register()

	// /metricsにアクセスがあった際にSpeedtestを実行するハンドラー
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		collectMetrics(metrics)            // /metricsアクセス時にSpeedtestを実行
		promhttp.Handler().ServeHTTP(w, r) // Prometheusのハンドラーを呼び出してメトリクスを返す
	})

	log.Println("Starting speedtest exporter server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
