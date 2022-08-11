// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package nodes is the collector for nodes, pods, and optionally containers
package nodes

import (
	"context"
	"crypto/tls"
	"sync"
	"time"

	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/circonus"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config/defaults"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/k8s"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/nodes/collector"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/release"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Nodes struct {
	sync.Mutex
	config       *config.Cluster
	check        *circonus.Check
	log          zerolog.Logger
	apiTimelimit time.Duration
	nodeCC       bool
	running      bool
}

func New(cfg *config.Cluster, parentLog zerolog.Logger, check *circonus.Check, nodeCC bool) (*Nodes, error) {
	if cfg == nil {
		return nil, errors.New("invalid cluster config (nil)")
	}
	if check == nil {
		return nil, errors.New("invalid check (nil)")
	}

	nodes := &Nodes{
		config: cfg,
		check:  check,
		nodeCC: nodeCC,
		log:    parentLog.With().Str("pkg", "nodes").Logger(),
	}

	if cfg.APITimelimit != "" {
		v, err := time.ParseDuration(cfg.APITimelimit)
		if err != nil {
			nodes.log.Error().Err(err).Msg("parsing api timelimit, using default")
		} else {
			nodes.apiTimelimit = v
		}
	}

	if nodes.apiTimelimit == time.Duration(0) {
		v, err := time.ParseDuration(defaults.K8SAPITimelimit)
		if err != nil {
			nodes.log.Fatal().Err(err).Msg("parsing DEFAULT api timelimit")
		}
		nodes.apiTimelimit = v
	}

	return nodes, nil
}

func (n *Nodes) ID() string {
	return "nodes"
}

func (n *Nodes) Collect(ctx context.Context, tlsConfig *tls.Config, ts *time.Time) {
	n.Lock()
	if n.running {
		n.log.Warn().Msg("already running")
		n.Unlock()
		return
	}
	n.running = true
	n.Unlock()

	defer func() {
		if r := recover(); r != nil {
			n.log.Error().Interface("panic", r).Msg("recover")
			n.Lock()
			n.running = false
			n.Unlock()
		}
	}()

	collectStart := time.Now()

	nodes, err := n.nodeList(ctx)
	if err != nil {
		n.log.Error().Err(err).Msg("fetching list of nodes")
		n.Lock()
		n.running = false
		n.Unlock()
		return
	}

	maxCollectors := int(n.config.NodePoolSize)
	nodeQueue := make(chan *collector.Collector)
	var wg sync.WaitGroup
	n.log.Debug().
		Int("num_workers", maxCollectors).
		Msg("starting node collectors")

	for i := 0; i < maxCollectors; i++ {
		wg.Add(1)
		id := i
		go func(nodeQueue chan *collector.Collector, id int) {
			defer wg.Done()
			workStart := time.Now()
			n.log.Debug().
				Int("worker_id", id).
				Msg("worker started")
			for node := range nodeQueue {
				node.Collect(ctx, id, tlsConfig, ts, n.nodeCC)
			}
			n.log.Debug().
				Str("duration", time.Since(workStart).String()).
				Int("worker_id", id).
				Msg("worker completed")
		}(nodeQueue, id)
	}

	n.check.AddGauge("collect_k8s_node_count", cgm.Tags{cgm.Tag{Category: "source", Value: release.NAME}}, len(nodes.Items))
	nodesQueued := 0
	for _, node := range nodes.Items {
		node := node
		for _, cond := range node.Status.Conditions {
			if cond.Type != v1.NodeReady {
				continue
			}
			if cond.Status == v1.ConditionTrue {
				nc, err := collector.New(n.config, &node, n.log, n.check, n.apiTimelimit)
				if err != nil {
					n.log.Error().Err(err).Str("node", node.Name).Msg("skipping...")
					break
				}
				nodeQueue <- nc
				nodesQueued++
			} else {
				n.log.Warn().Str(string(cond.Type), string(cond.Status)).Str("node", node.Name).Msg("skipping...")
			}
			break
		}
	}
	close(nodeQueue)
	wg.Wait() // wait for last one to finish

	n.check.AddHistSample("collect_latency", cgm.Tags{
		cgm.Tag{Category: "type", Value: "collect_nodes"},
		cgm.Tag{Category: "source", Value: release.NAME},
		cgm.Tag{Category: "units", Value: "milliseconds"},
	}, float64(time.Since(collectStart).Milliseconds()))

	n.log.Debug().
		Str("duration", time.Since(collectStart).String()).
		Int("nodes_queued", nodesQueued).
		Int("nodes_total", len(nodes.Items)).
		Int("node_workers", maxCollectors).
		Msg("node collect end")
	n.Lock()
	n.running = false
	n.Unlock()
}

func (n *Nodes) nodeList(ctx context.Context) (*v1.NodeList, error) {
	clientset, err := k8s.GetClient(n.config)
	if err != nil {
		n.log.Error().Err(err).Msg("initializing client set")
		return nil, err
	}

	listOptions := metav1.ListOptions{}

	if labelSelector := n.config.NodeSelector; labelSelector != "" {
		listOptions.LabelSelector = labelSelector
	}

	start := time.Now()
	nodes, err := clientset.CoreV1().Nodes().List(ctx, listOptions)
	if err != nil {
		n.check.IncrementCounter("collect_api_errors", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "request", Value: "node-list"},
			cgm.Tag{Category: "target", Value: "api-server"},
		})
		return nil, err
	}
	n.check.AddHistSample("collect_latency", cgm.Tags{
		cgm.Tag{Category: "source", Value: release.NAME},
		cgm.Tag{Category: "request", Value: "node-list"},
		cgm.Tag{Category: "target", Value: "api-server"},
		cgm.Tag{Category: "units", Value: "milliseconds"},
	}, float64(time.Since(start).Milliseconds()))

	if len(nodes.Items) == 0 {
		return nil, errors.New("zero nodes found, nothing to collect")
	}

	return nodes, nil
}
