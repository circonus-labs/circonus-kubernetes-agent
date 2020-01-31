// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package agent defines the main process
package agent

import (
	"context"
	"net/http"
	"os"
	"os/signal"

	"github.com/circonus-labs/circonus-kubernetes-agent/internal/cluster"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config/defaults"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config/keys"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/release"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
)

// Agent holds the main process
type Agent struct {
	group       *errgroup.Group
	groupCtx    context.Context
	groupCancel context.CancelFunc
	clusters    map[string]*cluster.Cluster
	signalCh    chan os.Signal
	logger      zerolog.Logger
}

// New returns a new agent instance
func New() (*Agent, error) {
	ctx, cancel := context.WithCancel(context.Background())
	g, gctx := errgroup.WithContext(ctx)

	var err error
	a := Agent{
		group:       g,
		groupCtx:    gctx,
		groupCancel: cancel,
		clusters:    make(map[string]*cluster.Cluster),
		signalCh:    make(chan os.Signal, 10),
		logger:      log.With().Str("pkg", "agent").Logger(),
	}

	err = config.Validate()
	if err != nil {
		return nil, err
	}

	var cfg *config.Config

	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, errors.Wrap(err, "parsing config")
	}

	// Set the hidden settings based on viper
	cfg.Circonus.Base64Tags = defaults.Base64Tags
	if viper.GetBool(keys.NoBase64) {
		cfg.Circonus.Base64Tags = false
	}
	cfg.Circonus.UseGZIP = defaults.UseGZIP
	if viper.GetBool(keys.NoGZIP) {
		cfg.Circonus.UseGZIP = false
	}
	cfg.Circonus.DryRun = viper.GetBool(keys.DryRun)
	cfg.Circonus.StreamMetrics = viper.GetBool(keys.StreamMetrics)
	cfg.Circonus.DebugSubmissions = viper.GetBool(keys.DebugSubmissions)

	if len(cfg.Clusters) > 0 { // multiple clusters
		for _, clusterConfig := range cfg.Clusters {
			clusterConfig := clusterConfig
			c, err := cluster.New(clusterConfig, cfg.Circonus, a.logger)
			if err != nil {
				a.logger.Error().Err(err).Msg("configuring cluster, skipping...")
				continue
			}
			a.clusters[clusterConfig.Name] = c
		}
	} else { // single cluster
		c, err := cluster.New(cfg.Kubernetes, cfg.Circonus, a.logger)
		if err != nil {
			a.logger.Error().Err(err).Msg("configuring cluster")
		} else {
			a.clusters[cfg.Kubernetes.Name] = c
		}
	}

	if len(a.clusters) == 0 {
		log.Fatal().Msg("no cluster(s) configured, must configure at least ONE")
	}

	a.signalNotifySetup()

	go func() {
		err := http.ListenAndServe(":8080", nil)
		if err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("internal http server exited")
		}
		// TODO: add additional handlers for health and readiness checks from k8s
		// NOTE: http://addr:8080/debug/vars for application information
		//       http://addr:8080/health TBD
		//       http://addr:8080/ready TBD
	}()

	return &a, nil
}

// Start the agent
func (a *Agent) Start() error {

	a.group.Go(a.handleSignals)

	for id := range a.clusters {
		id := id
		a.group.Go(func() error {
			return a.clusters[id].Start(a.groupCtx)
		})
	}

	log.Debug().
		Int("pid", os.Getpid()).
		Str("name", release.NAME).
		Str("ver", release.VERSION).Msg("starting wait")

	return a.group.Wait()
}

// Stop cleans up and shuts down the Agent
func (a *Agent) Stop() {
	a.stopSignalHandler()
	a.groupCancel()

	log.Debug().
		Int("pid", os.Getpid()).
		Str("name", release.NAME).
		Str("ver", release.VERSION).Msg("stopped")
}

// stopSignalHandler disables the signal handler
func (a *Agent) stopSignalHandler() {
	signal.Stop(a.signalCh)
	signal.Reset() // so a second ctrl-c will force immediate stop
}
