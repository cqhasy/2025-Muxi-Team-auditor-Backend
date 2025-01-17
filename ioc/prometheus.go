package ioc

import (
	"github.com/prometheus/client_golang/prometheus"
	"muxi_auditor/config"
	"muxi_auditor/pkg/prometheusx"
)

type Prometheus struct {
	RouterCounter     *prometheus.CounterVec
	ActiveConnections *prometheus.GaugeVec
	DurationTime      *prometheus.HistogramVec
}

// 感觉划分上不是特别的优雅,但是暂时没更好的办法
func InitPrometheus(conf *config.PrometheusConfig) *Prometheus {
	p := prometheusx.NewPrometheus(conf.Namespace)
	return &Prometheus{
		RouterCounter:     p.RegisterCounter(conf.RouterCounter.Name, conf.RouterCounter.Help, []string{"method", "endpoint", "status"}),
		ActiveConnections: p.RegisterGauge(conf.ActiveConnections.Name, conf.RouterCounter.Help, []string{"endpoint"}),
		DurationTime:      p.RegisterHistogram(conf.DurationTime.Name, conf.DurationTime.Help, []string{"endpoint", "status"}, prometheus.DefBuckets),
	}
}
