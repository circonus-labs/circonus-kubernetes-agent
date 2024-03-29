// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package as is the api-server collector
package as

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
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
	"github.com/prometheus/common/expfmt"
	"github.com/rs/zerolog"
)

type AS struct {
	sync.Mutex
	config       *config.Cluster
	check        *circonus.Check
	log          zerolog.Logger
	apiTimelimit time.Duration
	running      bool
}

func New(cfg *config.Cluster, parentLog zerolog.Logger, check *circonus.Check) (*AS, error) {
	if cfg == nil {
		return nil, errors.New("invalid cluster config (nil)")
	}
	if check == nil {
		return nil, errors.New("invalid check (nil)")
	}

	as := &AS{
		config: cfg,
		check:  check,
		log:    parentLog.With().Str("collector", "api-server").Logger(),
	}

	if cfg.APITimelimit != "" {
		v, err := time.ParseDuration(cfg.APITimelimit)
		if err != nil {
			as.log.Error().Err(err).Msg("parsing api timelimit, using default")
		} else {
			as.apiTimelimit = v
		}
	}

	if as.apiTimelimit == time.Duration(0) {
		v, err := time.ParseDuration(defaults.K8SAPITimelimit)
		if err != nil {
			as.log.Fatal().Err(err).Msg("parsing DEFAULT api timelimit")
		}
		as.apiTimelimit = v
	}

	return as, nil
}

func (as *AS) ID() string {
	return "api-server"
}

func (as *AS) Collect(ctx context.Context, tlsConfig *tls.Config, ts *time.Time) {
	as.Lock()
	if as.running {
		as.log.Warn().Msg("already running")
		as.Unlock()
		return
	}
	as.running = true
	as.Unlock()

	defer func() {
		if r := recover(); r != nil {
			as.log.Error().Interface("panic", r).Msg("recover")
			as.Lock()
			as.running = false
			as.Unlock()
		}
	}()

	collectStart := time.Now()

	clientset, err := k8s.GetClient(as.config)
	if err != nil {
		as.log.Error().Err(err).Msg("initializing client set")
		return
	}

	start := time.Now()
	req := clientset.CoreV1().RESTClient().Get().RequestURI("/metrics")
	res := req.Do(ctx)

	data, err := res.Raw()
	if err != nil {
		as.check.IncrementCounter("collect_api_errors", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "request", Value: "metrics"},
			cgm.Tag{Category: "target", Value: "api-server"},
		})
		as.log.Error().Err(err).Str("url", req.URL().String()).Msg("metrics")
		as.Lock()
		as.running = false
		as.Unlock()
		return
	}

	as.check.AddHistSample("collect_latency", cgm.Tags{
		cgm.Tag{Category: "source", Value: release.NAME},
		cgm.Tag{Category: "request", Value: "metrics"},
		cgm.Tag{Category: "target", Value: "api-server"},
		cgm.Tag{Category: "units", Value: "milliseconds"},
	}, float64(time.Since(start).Milliseconds()))

	var sc int
	res.StatusCode(&sc)
	if sc != http.StatusOK {
		as.check.IncrementCounter("collect_api_errors", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "request", Value: "metrics"},
			cgm.Tag{Category: "target", Value: "api-server"},
			cgm.Tag{Category: "code", Value: fmt.Sprintf("%d", sc)},
		})
		as.log.Warn().Str("url", req.URL().String()).Int("status", sc).Str("response", string(data)).Msg("error from AS server")
		return
	}

	streamTags := []string{
		"source:api-server",
		"__rollup:false", // prevent high cardinality metrics from rolling up
	}
	measurementTags := []string{}

	var parser expfmt.TextParser
	if err := promtext.QueueMetrics(ctx, parser, as.check, as.log, bytes.NewReader(data), streamTags, measurementTags, ts); err != nil {
		as.log.Error().Err(err).Msg("formatting metrics")
	}

	as.check.AddHistSample("collect_latency", cgm.Tags{
		cgm.Tag{Category: "source", Value: release.NAME},
		cgm.Tag{Category: "opt", Value: "collect_api-server"},
		cgm.Tag{Category: "units", Value: "milliseconds"},
	}, float64(time.Since(collectStart).Milliseconds()))
	as.log.Debug().Str("duration", time.Since(collectStart).String()).Msg("api-server collect end")
	as.Lock()
	as.running = false
	as.Unlock()
}
