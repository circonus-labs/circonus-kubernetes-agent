// Copyright © 2021 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package collector

import (
	"bytes"
	"time"

	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/k8s"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/promtext"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/release"
	"github.com/prometheus/common/expfmt"
)

// collector v2 methods (for k8s >= v1.20)
// NOTE: may actually be v1.18 where "Default the --enable-cadvisor-json-endpoints flag to disabled"
// NOTE: v1.21 "Remove the --enable-cadvisor-json-endpoints flag"

// resources emits node resources stats
func (nc *Collector) resources(parentStreamTags []string, parentMeasurementTags []string) {
	if nc.done() {
		return
	}

	start := time.Now()
	logger := nc.log.With().Str("type", "/metrics/resource").Logger()
	logger.Debug().Msg("start")

	clientset, err := k8s.GetClient(&nc.cfg)
	if err != nil {
		logger.Error().Err(err).Msg("initializing client set for node resource metrics, abandoning collection")
		logger.Debug().Str("duration", time.Since(start).String()).Msg("complete")
		return
	}

	req := clientset.CoreV1().RESTClient().Get().RequestURI(nc.baseURI + "/proxy/metrics/resource")
	res := req.Do(nc.ctx)
	data, err := res.Raw()
	if err != nil {
		nc.check.IncrementCounter("collect_api_errors", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "request", Value: "metrics/resource"},
			cgm.Tag{Category: "proxy", Value: "api-server"},
			cgm.Tag{Category: "target", Value: "kubelet"},
		})
		logger.Error().Err(err).Str("url", req.URL().String()).Msg("fetching /metrics/resource")
		logger.Debug().Str("duration", time.Since(start).String()).Msg("complete")
		return
	}

	nc.check.AddHistSample("collect_latency", cgm.Tags{
		cgm.Tag{Category: "source", Value: release.NAME},
		cgm.Tag{Category: "request", Value: "metrics/resource"},
		cgm.Tag{Category: "proxy", Value: "api-server"},
		cgm.Tag{Category: "target", Value: "kubelet"},
		cgm.Tag{Category: "units", Value: "milliseconds"},
	}, float64(time.Since(start).Milliseconds()))

	var parser expfmt.TextParser
	if err := promtext.QueueMetrics(nc.ctx, parser, nc.check, logger, bytes.NewReader(data), parentStreamTags, parentMeasurementTags, nil); err != nil {
		logger.Error().Err(err).Msg("parsing node resource metrics")
	}
	logger.Debug().Str("duration", time.Since(start).String()).Msg("complete")
}

// probes emits node probe stats
func (nc *Collector) probes(parentStreamTags []string, parentMeasurementTags []string) {
	if nc.done() {
		return
	}

	start := time.Now()
	logger := nc.log.With().Str("type", "/metrics/probes").Logger()
	logger.Debug().Msg("start")

	clientset, err := k8s.GetClient(&nc.cfg)
	if err != nil {
		logger.Error().Err(err).Msg("initializing client set for node probe metrics, abandoning collection")
		logger.Debug().Str("duration", time.Since(start).String()).Msg("complete")
		return
	}

	req := clientset.CoreV1().RESTClient().Get().RequestURI(nc.baseURI + "/proxy/metrics/probes")
	res := req.Do(nc.ctx)
	data, err := res.Raw()
	if err != nil {
		nc.check.IncrementCounter("collect_api_errors", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "request", Value: "metrics/probes"},
			cgm.Tag{Category: "proxy", Value: "api-server"},
			cgm.Tag{Category: "target", Value: "kubelet"},
		})
		logger.Error().Err(err).Str("url", req.URL().String()).Msg("fetching /metrics/probes")
		logger.Debug().Str("duration", time.Since(start).String()).Msg("complete")
		return
	}

	nc.check.AddHistSample("collect_latency", cgm.Tags{
		cgm.Tag{Category: "source", Value: release.NAME},
		cgm.Tag{Category: "request", Value: "metrics/probes"},
		cgm.Tag{Category: "proxy", Value: "api-server"},
		cgm.Tag{Category: "target", Value: "kubelet"},
		cgm.Tag{Category: "units", Value: "milliseconds"},
	}, float64(time.Since(start).Milliseconds()))

	var parser expfmt.TextParser
	if err := promtext.QueueMetrics(nc.ctx, parser, nc.check, logger, bytes.NewReader(data), parentStreamTags, parentMeasurementTags, nil); err != nil {
		logger.Error().Err(err).Msg("parsing node probe metrics")
	}
	logger.Debug().Str("duration", time.Since(start).String()).Msg("complete")
}
