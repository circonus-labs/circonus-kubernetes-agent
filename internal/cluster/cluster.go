// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package cluster is the collection manager for a cluster
package cluster

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"

	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/as"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/circonus"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/dc"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/dns"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/events"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/health"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/k8s"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/ksm"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/nodes"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/release"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type Cluster struct {
	sync.Mutex
	tlsConfig  *tls.Config
	check      *circonus.Check
	lastStart  *time.Time
	logger     zerolog.Logger
	collectors []string
	circCfg    config.Circonus
	cfg        config.Cluster
	interval   time.Duration
	running    bool
}
type Collector interface {
	ID() string
	Collect(context.Context, *tls.Config, *time.Time)
}

func New(cfg config.Cluster, circCfg config.Circonus, parentLog zerolog.Logger) (*Cluster, error) {
	if cfg.Name == "" {
		return nil, errors.New("invalid cluster config (empty name)")
	}
	if cfg.BearerToken == "" && cfg.BearerTokenFile == "" {
		return nil, errors.New("invalid bearer credentials (empty)")
	}

	c := &Cluster{
		cfg:        cfg,
		circCfg:    circCfg,
		collectors: []string{"health"},
		logger:     parentLog.With().Str("pkg", "cluster").Str("cluster_name", cfg.Name).Logger(),
	}

	if c.cfg.BearerToken == "" && c.cfg.BearerTokenFile != "" {
		token, err := ioutil.ReadFile(c.cfg.BearerTokenFile)
		if err != nil {
			return nil, errors.Wrap(err, "bearer token file")
		}
		c.cfg.BearerToken = string(token)
	}
	c.logger.Debug().Str("token", c.cfg.BearerToken[0:8]+"...").Msg("using bearer token")

	if c.cfg.CAFile != "" {
		cert, err := ioutil.ReadFile(c.cfg.CAFile)
		if err != nil {
			return nil, errors.Wrap(err, "configuring k8s api tls")
		}
		cp := x509.NewCertPool()
		if !cp.AppendCertsFromPEM(cert) {
			return nil, errors.New("unable to add k8s api CA Certificate to x509 cert pool")
		}
		c.tlsConfig = &tls.Config{
			RootCAs:    cp,
			MinVersion: tls.VersionTLS12,
			// InsecureSkipVerify: true,
		}
		c.logger.Debug().Str("cert", c.cfg.CAFile).Msg("adding CA cert to TLS config")
	}

	d, err := time.ParseDuration(c.cfg.Interval)
	if err != nil {
		return nil, errors.Wrap(err, "invalid duration in cluster configuration")
	}
	c.interval = d
	c.logger.Debug().Str("interval", d.String()).Msg("using interval")

	// set check title if it has not been explicitly set by user
	if circCfg.Check.Title == "" {
		circCfg.Check.Title = fmt.Sprintf("%s /%s", cfg.Name, release.NAME)
	}

	if circCfg.Check.Target == "" {
		circCfg.Check.Target = strings.ReplaceAll(cfg.Name, " ", "_")
	}
	check, err := circonus.NewCheck(c.logger, &circCfg, &cfg)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to initialize circonus for cluster (%s)", cfg.Name)
	}
	c.check = check

	if c.cfg.EnableKubeStateMetrics {
		c.collectors = append(c.collectors, "ksm")
	}

	if c.cfg.EnableAPIServer {
		c.collectors = append(c.collectors, "api")
	}

	if c.cfg.EnableDNSMetrics {
		c.collectors = append(c.collectors, "dns")
	}

	if c.cfg.EnableNodes {
		// node metrics, as well as, pod and container metrics (both optional)
		c.collectors = append(c.collectors, "node")
	}

	if len(c.collectors) == 0 {
		return nil, errors.Errorf("no collectors enabled for cluster %s", c.cfg.Name)
	}

	return c, nil
}

func (c *Cluster) Start(ctx context.Context) error {

	var eventWatcher *events.Events
	if c.cfg.EnableEvents {
		ew, err := events.New(&c.cfg, c.logger, c.check)
		if err != nil {
			return errors.Wrap(err, "initializing events collector")
		}
		eventWatcher = ew
	}

	var dynamicCollectors *dc.DC
	if c.cfg.DynamicCollectorFile != "" {
		var err error
		d, err := dc.New(&c.cfg, c.logger, c.check)
		if err != nil {
			c.logger.Warn().Err(err).Msg("initializing dynamic collectors, disabling")
		} else {
			dynamicCollectors = d
		}
	}

	if len(c.collectors) == 0 && eventWatcher == nil && dynamicCollectors == nil {
		return errors.New("invalid cluster (zero collectors)")
	}

	if eventWatcher != nil {
		go eventWatcher.Start(ctx, c.tlsConfig)
	}

	c.collect(ctx, dynamicCollectors)

	c.logger.Info().Str("collection_interval", c.interval.String()).Time("next_collection", time.Now().Add(c.interval)).Msg("client started")

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			c.Lock()
			if c.lastStart != nil {
				elapsed := time.Since(*c.lastStart)
				if c.interval.Round(time.Second)-elapsed.Round(time.Second) > 2 {
					c.Unlock()
					c.logger.Warn().
						Str("last_start", c.lastStart.String()).
						Dur("elapsed", elapsed).
						Dur("interval", c.interval).
						Msg("interval not reached")
					continue
				}
			}
			if c.running {
				c.Unlock()
				c.logger.Warn().
					Str("started", c.lastStart.String()).
					Str("elapsed", time.Since(*c.lastStart).String()).
					Msg("collection in progress, not starting another")
				continue
			}
			c.Unlock()

			go func() {
				c.collect(ctx, dynamicCollectors)
			}()
		}
	}
}

func (c *Cluster) collect(ctx context.Context, dynamicCollectors *dc.DC) {
	c.Lock()
	start := time.Now()
	c.lastStart = &start
	c.running = true
	c.Unlock()

	collectCtx, collectCancel := context.WithDeadline(ctx, time.Now().Add(c.interval))
	defer collectCancel()

	// reset submit retries metric
	c.check.SetCounter("collect_submit_retries", cgm.Tags{cgm.Tag{Category: "source", Value: release.NAME}}, 0)

	var wg sync.WaitGroup

	if dynamicCollectors != nil {
		wg.Add(1)
		go func() {
			dynamicCollectors.Collect(collectCtx, c.tlsConfig, &start)
			wg.Done()
		}()
	}

	for _, collectorID := range c.collectors {
		switch collectorID {
		case "node":
			collector, err := nodes.New(&c.cfg, c.logger, c.check, c.circCfg.NodeCC)
			if err != nil {
				c.logger.Error().Err(err).Msg("initializing node collector")
			} else {
				collector.Collect(collectCtx, c.tlsConfig, &start)
			}
		case "health":
			wg.Add(1)
			go func() {
				collector, err := health.New(&c.cfg, c.logger, c.check)
				if err != nil {
					c.logger.Error().Err(err).Msg("initializing health collector")
				} else {
					collector.Collect(collectCtx, c.tlsConfig, &start)
				}
				wg.Done()
			}()
		case "ksm":
			wg.Add(1)
			go func() {
				collector, err := ksm.New(&c.cfg, c.logger, c.check)
				if err != nil {
					c.logger.Error().Err(err).Msg("initializing kube-state-metrics collector")
				} else {
					collector.Collect(collectCtx, c.tlsConfig, &start)
				}
				wg.Done()
			}()
		case "api":
			wg.Add(1)
			go func() {
				collector, err := as.New(&c.cfg, c.logger, c.check)
				if err != nil {
					c.logger.Error().Err(err).Msg("initializing api-server collector")
				} else {
					collector.Collect(collectCtx, c.tlsConfig, &start)
				}
				wg.Done()
			}()
		case "dns":
			wg.Add(1)
			go func() {
				collector, err := dns.New(&c.cfg, c.logger, c.check)
				if err != nil {
					c.logger.Error().Err(err).Msg("initializing kube-dns/coredns collector")
				} else {
					collector.Collect(collectCtx, c.tlsConfig, &start)
				}
				wg.Done()
			}()
		default:
			c.logger.Warn().Str("collector_id", collectorID).Msg("ignoring unknown collector")
		}
	}

	wg.Wait()

	deadlineTimeout := false
	select {
	case <-collectCtx.Done():
		c.logger.Warn().Msg("deadline triggered cancellation of metric collection: increase interval, node pool, or resources")
		deadlineTimeout = true
	default:
	}

	{ // get api/cluster version/platform

		verplat, err := k8s.GetVersionPlatform(&c.cfg)
		if err != nil {
			c.logger.Warn().Err(err).Msg("getting api/cluster version + platform information")
		}
		c.check.AddText("collect_k8s_ver", cgm.Tags{cgm.Tag{Category: "source", Value: release.NAME}}, verplat)
	}

	cstats := c.check.SubmitStats()
	c.check.ResetSubmitStats()
	dur := time.Since(start)

	baseStreamTags := cgm.Tags{
		cgm.Tag{Category: "cluster", Value: c.cfg.Name},
		cgm.Tag{Category: "source", Value: release.NAME},
	}
	c.check.AddText("collect_agent", baseStreamTags, release.NAME+"_"+release.VERSION)
	c.check.AddGauge("collect_metrics", baseStreamTags, cstats.SentMetrics)
	c.check.AddGauge("collect_filtered", baseStreamTags, cstats.LocFiltered)
	c.check.AddGauge("collect_ngr", baseStreamTags, uint64(runtime.NumGoroutine()))

	cdt := 0
	if deadlineTimeout {
		cdt = 1
	}
	c.check.AddGauge("collect_deadline_timeout", baseStreamTags, cdt)

	{
		debug.FreeOSMemory()

		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)

		c.check.AddGauge("collect_mem_frag", baseStreamTags, float64(ms.Sys-ms.HeapReleased)/float64(ms.HeapInuse))
		c.check.AddGauge("collect_numgc", baseStreamTags, ms.NumGC)
		c.check.AddGauge("collect_heap_objs", baseStreamTags, ms.HeapObjects)
		c.check.AddGauge("collect_live_obj", baseStreamTags, ms.Mallocs-ms.Frees)

		var streamTags cgm.Tags
		streamTags = append(streamTags, baseStreamTags...)
		streamTags = append(streamTags, cgm.Tag{Category: "units", Value: "bytes"})

		c.check.AddGauge("collect_sent", streamTags, cstats.SentBytes)
		c.check.AddGauge("collect_heap_alloc", streamTags, ms.HeapAlloc)
		c.check.AddGauge("collect_heap_inuse", streamTags, ms.HeapInuse)
		c.check.AddGauge("collect_heap_idle", streamTags, ms.HeapIdle)
		c.check.AddGauge("collect_heap_released", streamTags, ms.HeapReleased)
		c.check.AddGauge("collect_stack_sys", streamTags, ms.StackSys)
		c.check.AddGauge("collect_other_sys", streamTags, ms.OtherSys)

		var mem syscall.Rusage
		if err := syscall.Getrusage(syscall.RUSAGE_SELF, &mem); err == nil {
			c.check.AddGauge("collect_max_rss", streamTags, uint64(mem.Maxrss*1024))
		} else {
			c.logger.Warn().Err(err).Msg("collecting rss from system")
		}
	}

	{
		var streamTags cgm.Tags
		streamTags = append(streamTags, baseStreamTags...)
		streamTags = append(streamTags, cgm.Tag{Category: "units", Value: "milliseconds"})
		c.check.AddGauge("collect_duration", streamTags, uint64(dur.Milliseconds()))
		c.check.AddGauge("collect_interval", streamTags, uint64(c.interval.Milliseconds()))
	}

	// use regular ctx not the collection deadlined ctx
	c.check.FlushCGM(ctx, &start, c.logger, true)

	c.logger.Info().
		Interface("metrics_sent", cstats).
		Str("duration", dur.String()).
		Msg("collection complete")
	c.Lock()
	c.running = false
	c.Unlock()
}
