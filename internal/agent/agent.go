// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package agent defines the main process
package agent

import (
	"context"
	"errors"
	"expvar"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/circonus-labs/circonus-kubernetes-agent/internal/cluster"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config/defaults"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config/keys"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/release"
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

func serveHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		switch r.URL.Path {
		case "/stats", "/stats/":
			expvar.Handler().ServeHTTP(w, r)
		case "/health", "/health/":
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "Alive")
		default:
			http.NotFound(w, r)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
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
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	a.logger.Info().
		Bool(keys.K8SEnableAPIServer, viper.GetBool(keys.K8SEnableAPIServer)).
		Bool(keys.K8SEnableCadvisorMetrics, viper.GetBool(keys.K8SEnableCadvisorMetrics)).
		Bool(keys.K8SEnableDNSMetrics, viper.GetBool(keys.K8SEnableDNSMetrics)).
		Int(keys.K8SDNSMetricsPort, viper.GetInt(keys.K8SDNSMetricsPort)).
		Bool(keys.K8SEnableEvents, viper.GetBool(keys.K8SEnableEvents)).
		Bool(keys.K8SEnableKubeStateMetrics, viper.GetBool(keys.K8SEnableKubeStateMetrics)).
		Bool(keys.K8SEnableNodeMetrics, viper.GetBool(keys.K8SEnableNodeMetrics)).
		Bool(keys.K8SEnableNodeProbeMetrics, viper.GetBool(keys.K8SEnableNodeProbeMetrics)).
		Bool(keys.K8SEnableNodeResourceMetrics, viper.GetBool(keys.K8SEnableNodeResourceMetrics)).
		Bool(keys.K8SEnableNodeStats, viper.GetBool(keys.K8SEnableNodeStats)).
		Bool(keys.K8SEnableNodes, viper.GetBool(keys.K8SEnableNodes)).
		Bool(keys.K8SIncludeContainers, viper.GetBool(keys.K8SIncludeContainers)).
		Bool(keys.K8SIncludePods, viper.GetBool(keys.K8SIncludePods)).
		Str(keys.K8SInterval, viper.GetString(keys.K8SInterval)).
		Str(keys.CollectDeadline, viper.GetString(keys.CollectDeadline)).
		Str(keys.SubmitDeadline, viper.GetString(keys.SubmitDeadline)).
		Msg("collection configuration")

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
	// cfg.Circonus.StreamMetrics = viper.GetBool(keys.StreamMetrics)
	cfg.Circonus.LogAgentMetrics = viper.GetBool(keys.LogAgentMetrics)

	cfg.Circonus.NodeCC = viper.GetBool(keys.NodeCC)

	if len(cfg.Clusters) > 0 { // multiple clusters
		for _, clusterConfig := range cfg.Clusters {
			clusterConfig := clusterConfig
			c, err := cluster.New(gctx, clusterConfig, cfg.Circonus, a.logger)
			if err != nil {
				a.logger.Error().Err(err).Msg("configuring cluster, skipping...")
				continue
			}
			a.clusters[clusterConfig.Name] = c
		}
	} else { // single cluster
		c, err := cluster.New(gctx, cfg.Kubernetes, cfg.Circonus, a.logger)
		if err != nil {
			a.logger.Error().Err(err).Msg("configuring cluster")
		} else {
			a.clusters[cfg.Kubernetes.Name] = c
		}
	}

	if len(a.clusters) == 0 {
		log.Fatal().Msg("no cluster(s) initialized")
	}

	a.signalNotifySetup()

	go func() {
		// _ = http.ListenAndServe(":6060", nil) // pprof
		// NOTE: http://addr:8080/stats - application stats
		//       http://addr:8080/health - liveness probe
		srv := http.Server{
			Addr:              ":8080",
			WriteTimeout:      10 * time.Second,
			ReadHeaderTimeout: 2 * time.Second,
			ReadTimeout:       10 * time.Second,
			Handler:           http.HandlerFunc(serveHTTP),
		}
		err := srv.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("internal http server exited")
		}
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
