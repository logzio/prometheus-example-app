package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var (
	appVersion string
	version    = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "version",
		Help: "Version information about this binary",
		ConstLabels: map[string]string{
			"version": appVersion,
		},
	})

	httpRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Count of all HTTP requests",
	}, []string{"code", "method"})

	httpRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "http_request_duration_seconds",
		Help: "Duration of all HTTP requests",
	}, []string{"code", "handler", "method"})

	logzerName = os.Getenv("LOGZER_NAME")
	logzerTenure, _ = strconv.ParseFloat(os.Getenv("LOGZER_TENURE_IN_DAYS"),64)

	logzerTenureMetric    = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "logzer_tenure_in_days",
		Help: "How long the Logzer has been here",
		ConstLabels: map[string]string{
			"name": logzerName,
		},
	})
)

func main() {
	version.Set(1)
	bind := ""
	enableH2c := false
	flagset := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flagset.StringVar(&bind, "bind", ":8080", "The socket to bind to.")
	flagset.BoolVar(&enableH2c, "h2c", false, "Enable h2c (http/2 over tcp) protocol.")
	flagset.Parse(os.Args[1:])

	logzerTenureMetric.Set(logzerTenure)

	r := prometheus.NewRegistry()
	r.MustRegister(httpRequestsTotal)
	r.MustRegister(httpRequestDuration)
	r.MustRegister(version)
	r.MustRegister(logzerTenureMetric)

	foundHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("%s has worked at Logz.io for %f days", logzerName, logzerTenure)))
	})
	notfoundHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	foundChain := promhttp.InstrumentHandlerDuration(
		httpRequestDuration.MustCurryWith(prometheus.Labels{"handler": "found"}),
		promhttp.InstrumentHandlerCounter(httpRequestsTotal, foundHandler),
	)

	mux := http.NewServeMux()
	mux.Handle("/", foundChain)
	mux.Handle("/err", promhttp.InstrumentHandlerCounter(httpRequestsTotal, notfoundHandler))
	mux.Handle("/metrics", promhttp.HandlerFor(r, promhttp.HandlerOpts{}))

	var srv *http.Server
	if enableH2c {
		srv = &http.Server{Addr: bind, Handler: h2c.NewHandler(mux, &http2.Server{})}
	} else {
		srv = &http.Server{Addr: bind, Handler: mux}
	}

	log.Fatal(srv.ListenAndServe())
}
