// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package ms is the metrics-server collector
package ms

import (
	"bytes"
	"context"
	"crypto/tls"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/circonus-labs/circonus-kubernetes-agent/internal/circonus"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/k8s"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/promtext"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type MS struct {
	config  *config.Cluster
	bufSize int
	check   *circonus.Check
	log     zerolog.Logger
	running bool
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
		config:  cfg,
		bufSize: 32768,
		check:   check,
		log:     parentLog.With().Str("collector", "metrics-server").Logger(),
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

	client, err := k8s.NewAPIClient(tlsConfig)
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

	resp, err := client.Do(req)
	if err != nil {
		ms.log.Error().Err(err).Str("url", metricsURL).Msg("metrics")
		ms.Lock()
		ms.running = false
		ms.Unlock()
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			ms.log.Error().Err(err).Str("url", metricsURL).Msg("reading response")
			return
		}
		ms.log.Warn().Str("url", metricsURL).Str("status", resp.Status).RawJSON("response", data).Msg("error from API server")
		return
	}

	var buf bytes.Buffer
	buf.Grow(ms.bufSize)

	streamTags := []string{"source:metrics-server"}
	measurementTags := []string{}

	if err := promtext.StreamMetrics(ctx, &buf, ms.log, resp.Body, ms.check, streamTags, measurementTags, ts); err != nil {
		ms.log.Error().Err(err).Msg("formatting metrics")
	}

	if buf.Len() > 0 {
		submitStart := time.Now()
		if err := ms.check.SubmitStream(&buf, ms.log); err != nil {
			ms.log.Warn().Err(err).Msg("submitting metrics")
		}
		ms.bufSize = buf.Len() // save for next allocation to minimize dynamic growth
		ms.log.Info().Str("duration", time.Since(submitStart).String()).Int("size", ms.bufSize).Msg("submit end")
	} else {
		ms.log.Warn().Msg("no telemetry to submit")
	}

	ms.log.Info().Str("duration", time.Since(collectStart).String()).Msg("metric-server collect end")
	ms.Lock()
	ms.running = false
	ms.Unlock()
}
