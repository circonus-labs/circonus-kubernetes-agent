// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package ms is the metrics-server collector
package ms

import (
	"context"
	"crypto/tls"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/circonus"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config/defaults"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/k8s"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/promtext"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/release"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type MS struct {
	config       *config.Cluster
	check        *circonus.Check
	log          zerolog.Logger
	running      bool
	apiTimelimit time.Duration
	sync.Mutex
}

// NOTES:
// determine if pod is even running in cluster:
// curl localhost:8080/api/v1/pods?labelSelector=k8s-app%3Dmetrics-server
// will return 200 but the PodList returned will have 0 items
// of course, requires it's still labeled "k8s-app:metrics-server"

func New(cfg *config.Cluster, parentLog zerolog.Logger, check *circonus.Check) (*MS, error) {
	if cfg == nil {
		return nil, errors.New("invalid cluster config (nil)")
	}
	if check == nil {
		return nil, errors.New("invalid check (nil)")
	}

	ms := &MS{
		config: cfg,
		check:  check,
		log:    parentLog.With().Str("collector", "metrics-server").Logger(),
	}

	if cfg.APITimelimit != "" {
		v, err := time.ParseDuration(cfg.APITimelimit)
		if err != nil {
			ms.log.Error().Err(err).Msg("parsing api timelimit, using default")
		} else {
			ms.apiTimelimit = v
		}
	}

	if ms.apiTimelimit == time.Duration(0) {
		v, err := time.ParseDuration(defaults.K8SAPITimelimit)
		if err != nil {
			ms.log.Fatal().Err(err).Msg("parsing DEFAULT api timelimit")
		}
		ms.apiTimelimit = v
	}

	return ms, nil
}

func (ms *MS) ID() string {
	return "metrics-server"
}

func (ms *MS) Collect(ctx context.Context, tlsConfig *tls.Config, ts *time.Time) {
	ms.Lock()
	if ms.running {
		ms.log.Warn().Msg("already running")
		ms.Unlock()
		return
	}
	ms.running = true
	ms.Unlock()

	defer func() {
		if r := recover(); r != nil {
			ms.log.Error().Interface("panic", r).Msg("recover")
			ms.Lock()
			ms.running = false
			ms.Unlock()
		}
	}()

	collectStart := time.Now()

	metricsURL := ms.config.URL + "/metrics"

	client, err := k8s.NewAPIClient(tlsConfig, ms.apiTimelimit)
	if err != nil {
		ms.log.Error().Err(err).Str("url", metricsURL).Msg("metrics cli")
		ms.Lock()
		ms.running = false
		ms.Unlock()
		return
	}
	defer client.CloseIdleConnections()

	req, err := k8s.NewAPIRequest(ms.config.BearerToken, metricsURL)
	if err != nil {
		ms.log.Error().Err(err).Str("url", metricsURL).Msg("metrics req")
		ms.Lock()
		ms.running = false
		ms.Unlock()
		return
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		ms.check.IncrementCounter("collect_api_errors", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "request", Value: "metrics"},
			cgm.Tag{Category: "proxy", Value: "api-server"},
			cgm.Tag{Category: "target", Value: "metric-server"}})
		ms.log.Error().Err(err).Str("url", metricsURL).Msg("metrics")
		ms.Lock()
		ms.running = false
		ms.Unlock()
		return
	}
	defer resp.Body.Close()
	ms.check.AddHistSample("collect_latency", cgm.Tags{
		cgm.Tag{Category: "source", Value: release.NAME},
		cgm.Tag{Category: "request", Value: "metrics"},
		cgm.Tag{Category: "proxy", Value: "api-server"},
		cgm.Tag{Category: "target", Value: "metric-server"},
		cgm.Tag{Category: "units", Value: "milliseconds"},
	}, float64(time.Since(start).Milliseconds()))

	if resp.StatusCode != http.StatusOK {
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			ms.log.Error().Err(err).Str("url", metricsURL).Msg("reading response")
			return
		}
		ms.log.Warn().Str("url", metricsURL).Str("status", resp.Status).RawJSON("response", data).Msg("error from API server")
		return
	}

	streamTags := []string{"source:metrics-server"}
	measurementTags := []string{}

	if ms.check.StreamMetrics() {
		if err := promtext.StreamMetrics(ctx, ms.check, ms.log, resp.Body, ms.check, streamTags, measurementTags, ts); err != nil {
			ms.log.Error().Err(err).Msg("formatting metrics")
		}
	} else {
		if err := promtext.QueueMetrics(ctx, ms.check, ms.log, resp.Body, ms.check, streamTags, measurementTags, ts); err != nil {
			ms.log.Error().Err(err).Msg("formatting metrics")
		}
	}

	ms.check.AddHistSample("collect_latency", cgm.Tags{
		cgm.Tag{Category: "source", Value: release.NAME},
		cgm.Tag{Category: "opt", Value: "collect_metrics-server"},
		cgm.Tag{Category: "units", Value: "milliseconds"},
	}, float64(time.Since(collectStart).Milliseconds()))
	ms.log.Debug().Str("duration", time.Since(collectStart).String()).Msg("metric-server collect end")
	ms.Lock()
	ms.running = false
	ms.Unlock()
}
