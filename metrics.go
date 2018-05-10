package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/spotahome/kooper/log"
	"github.com/spotahome/kooper/monitoring/metrics"
)

const (
	metricsAddr       = ":7777"
	prometheusBackend = "prometheus"
)

// creates prometheus recorder and starts serving metrics in background.
func createPrometheusRecorder(logger log.Logger, namespace string) metrics.Recorder {
	// We could use also prometheus global registry (the default one)
	// prometheus.DefaultRegisterer instead of creating a new one
	reg := prometheus.NewRegistry()
	m := metrics.NewPrometheus(namespace, reg)

	// Start serving metrics in background.
	h := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	go func() {
		logger.Infof("serving metrics at %s", metricsAddr)
		http.ListenAndServe(metricsAddr, h)
	}()

	return m
}
