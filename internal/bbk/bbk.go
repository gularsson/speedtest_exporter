package bbk

import (
	"bufio"
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

const (
	namespace      = "bbk"
	mbitsToBytes   = 125000 // 1 Mbit/s = 125000 bytes/s
	commandTimeout = 120 * time.Second
)

var (
	up = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "up"),
		"Was the last BBK measurement successful.",
		nil, nil,
	)
	scrapeDurationSeconds = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "scrape_duration_seconds"),
		"Time to perform last BBK measurement",
		nil, nil,
	)
	latency = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "latency_seconds"),
		"Measured latency on last BBK measurement",
		[]string{"server", "isp"},
		nil,
	)
	upload = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "upload_speed_Bps"),
		"Last BBK upload measurement result",
		[]string{"server", "isp"},
		nil,
	)
	download = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "download_speed_Bps"),
		"Last BBK download measurement result",
		[]string{"server", "isp"},
		nil,
	)
)

// Exporter runs BBK measurements and exports them using
// the prometheus metrics package.
type Exporter struct {
	binaryPath string
}

// New returns an initialized BBK Exporter.
func New(binaryPath string) *Exporter {
	return &Exporter{
		binaryPath: binaryPath,
	}
}

// Describe describes all the metrics. It implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- up
	ch <- scrapeDurationSeconds
	ch <- latency
	ch <- upload
	ch <- download
}

// Collect runs a BBK measurement and delivers the results
// as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()
	ok := e.measure(ch)

	if ok {
		ch <- prometheus.MustNewConstMetric(up, prometheus.GaugeValue, 1.0)
	} else {
		ch <- prometheus.MustNewConstMetric(up, prometheus.GaugeValue, 0.0)
	}
	ch <- prometheus.MustNewConstMetric(
		scrapeDurationSeconds, prometheus.GaugeValue, time.Since(start).Seconds(),
	)
}

func (e *Exporter) measure(ch chan<- prometheus.Metric) bool {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, e.binaryPath, "--quiet", "--ssl")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Errorf("bbk: failed to create stdout pipe: %s", err)
		return false
	}

	if err := cmd.Start(); err != nil {
		log.Errorf("bbk: failed to start: %s", err)
		return false
	}

	var resultLine string
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		resultLine = scanner.Text()
	}

	if err := cmd.Wait(); err != nil {
		log.Errorf("bbk: command failed: %s", err)
		return false
	}

	if resultLine == "" {
		log.Error("bbk: no output from measurement")
		return false
	}

	return parseQuietOutput(resultLine, ch)
}

// parseQuietOutput parses BBK --quiet output.
// Format: download upload latency server isp ticket measurement_id [rating]
func parseQuietOutput(line string, ch chan<- prometheus.Metric) bool {
	fields := strings.Fields(line)
	if len(fields) < 5 {
		log.Errorf("bbk: unexpected output format, got %d fields: %q", len(fields), line)
		return false
	}

	dlMbits, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		log.Errorf("bbk: failed to parse download value %q: %s", fields[0], err)
		return false
	}

	ulMbits, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		log.Errorf("bbk: failed to parse upload value %q: %s", fields[1], err)
		return false
	}

	latencyMs, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		log.Errorf("bbk: failed to parse latency value %q: %s", fields[2], err)
		return false
	}

	server := fields[3]
	isp := fields[4]

	ch <- prometheus.MustNewConstMetric(
		download, prometheus.GaugeValue, dlMbits*mbitsToBytes,
		server, isp,
	)
	ch <- prometheus.MustNewConstMetric(
		upload, prometheus.GaugeValue, ulMbits*mbitsToBytes,
		server, isp,
	)
	ch <- prometheus.MustNewConstMetric(
		latency, prometheus.GaugeValue, latencyMs/1000.0,
		server, isp,
	)

	return true
}
