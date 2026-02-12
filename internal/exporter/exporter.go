package exporter

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/showwin/speedtest-go/speedtest"
	log "github.com/sirupsen/logrus"
)

const (
	namespace = "speedtest"
)

var (
	up = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "up"),
		"Was the last speedtest successful.",
		nil, nil,
	)
	scrapeDurationSeconds = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "scrape_duration_seconds"),
		"Time to perform last speed test",
		nil, nil,
	)
	latency = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "latency_seconds"),
		"Measured latency on last speed test",
		[]string{"server_id", "server_name", "server_country", "user_isp"},
		nil,
	)
	upload = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "upload_speed_Bps"),
		"Last upload speedtest result",
		[]string{"server_id", "server_name", "server_country", "user_isp"},
		nil,
	)
	download = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "download_speed_Bps"),
		"Last download speedtest result",
		[]string{"server_id", "server_name", "server_country", "user_isp"},
		nil,
	)
)

// Exporter runs speedtest and exports them using
// the prometheus metrics package.
type Exporter struct {
	serverID       int
	serverFallback bool
}

// New returns an initialized Exporter.
func New(serverID int, serverFallback bool) (*Exporter, error) {
	return &Exporter{
		serverID:       serverID,
		serverFallback: serverFallback,
	}, nil
}

// Describe describes all the metrics. It implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- up
	ch <- scrapeDurationSeconds
	ch <- latency
	ch <- upload
	ch <- download
}

// Collect fetches the stats from Starlink dish and delivers them
// as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()
	ok := e.speedtest(ch)

	if ok {
		ch <- prometheus.MustNewConstMetric(
			up, prometheus.GaugeValue, 1.0,
		)
	} else {
		ch <- prometheus.MustNewConstMetric(
			up, prometheus.GaugeValue, 0.0,
		)
	}
	ch <- prometheus.MustNewConstMetric(
		scrapeDurationSeconds, prometheus.GaugeValue, time.Since(start).Seconds(),
	)
}

func (e *Exporter) speedtest(ch chan<- prometheus.Metric) bool {
	user, err := speedtest.FetchUserInfo()
	if err != nil {
		log.Errorf("could not fetch user information: %s", err.Error())
		return false
	}

	// returns list of servers in distance order
	servers, err := speedtest.FetchServers()
	if err != nil {
		log.Errorf("could not fetch server list: %s", err.Error())
		return false
	}

	var server *speedtest.Server

	if len(servers) == 0 {
		log.Error("no servers found")
		return false
	}

	if e.serverID == -1 {
		server = servers[0]
	} else {
		found, err := servers.FindServer([]int{e.serverID})
		if err != nil {
			log.Error(err)
			return false
		}

		if len(found) == 0 {
			log.Errorf("could not find server ID %d in the list of available servers", e.serverID)
			return false
		}

		if found[0].ID != strconv.Itoa(e.serverID) && !e.serverFallback {
			log.Errorf("could not find your chosen server ID %d in the list of available servers, server_fallback is not set so failing this test", e.serverID)
			return false
		}

		server = found[0]
	}

	ok := pingTest(user, server, ch)
	ok = downloadTest(user, server, ch) && ok
	ok = uploadTest(user, server, ch) && ok

	return ok
}

func pingTest(user *speedtest.User, server *speedtest.Server, ch chan<- prometheus.Metric) bool {
	err := server.PingTest(func(latency time.Duration) {})
	if err != nil {
		log.Errorf("failed to carry out ping test: %s", err.Error())
		return false
	}

	ch <- prometheus.MustNewConstMetric(
		latency, prometheus.GaugeValue, server.Latency.Seconds(),
		server.ID,
		server.Name,
		server.Country,
		user.Isp,
	)

	return true
}

func downloadTest(user *speedtest.User, server *speedtest.Server, ch chan<- prometheus.Metric) bool {
	err := server.DownloadTest()
	if err != nil {
		log.Errorf("failed to carry out download test: %s", err.Error())
		return false
	}

	ch <- prometheus.MustNewConstMetric(
		download, prometheus.GaugeValue, float64(server.DLSpeed),
		server.ID,
		server.Name,
		server.Country,
		user.Isp,
	)

	return true
}

func uploadTest(user *speedtest.User, server *speedtest.Server, ch chan<- prometheus.Metric) bool {
	err := server.UploadTest()
	if err != nil {
		log.Errorf("failed to carry out upload test: %s", err.Error())
		return false
	}

	ch <- prometheus.MustNewConstMetric(
		upload, prometheus.GaugeValue, float64(server.ULSpeed),
		server.ID,
		server.Name,
		server.Country,
		user.Isp,
	)

	return true
}
