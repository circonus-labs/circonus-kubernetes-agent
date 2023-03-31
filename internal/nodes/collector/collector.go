// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package collector collects metrics from nodes
package collector

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"sync"
	"time"

	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/circonus"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config/keys"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/release"
	"github.com/hashicorp/go-version"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type Collector struct {
	ctx           context.Context
	tlsConfig     *tls.Config
	check         *circonus.Check
	node          *v1.Node
	kubeletVer    *version.Version
	ts            *time.Time
	baseURI       string
	baseLogger    zerolog.Logger
	log           zerolog.Logger
	verConstraint version.Constraints
	cfg           config.Cluster
	apiTimelimit  time.Duration
}

func New(cfg *config.Cluster, node *v1.Node, logger zerolog.Logger, check *circonus.Check, apiTimeout time.Duration) (*Collector, error) {
	if cfg == nil {
		return nil, fmt.Errorf("invalid cluster config (nil)")
	}
	if node == nil {
		return nil, fmt.Errorf("invalid node (nil)")
	}
	if check == nil {
		return nil, fmt.Errorf("invalid check (nil)")
	}

	c := &Collector{
		cfg:          *cfg, // make sure it's a copy
		check:        check,
		node:         node,
		apiTimelimit: apiTimeout,
		baseLogger:   logger.With().Str("node", node.Name).Logger(),
	}

	ver := node.Status.NodeInfo.KubeletVersion
	b4, _, found := strings.Cut(ver, "-")
	if found {
		ver = b4
	}

	v, err := version.NewVersion(ver)
	if err != nil {
		return nil, fmt.Errorf("parsing version (%s): %w", ver, err)
	}

	minVer := viper.GetString(keys.K8SNodeKubeletVersion)
	vc, err := version.NewConstraint(">= " + minVer)
	if err != nil {
		return nil, fmt.Errorf("parsing ver constraint: %w", err)
	}

	c.verConstraint = vc
	c.kubeletVer = v

	sl := "/api/v1/nodes/" + node.Name

	c.baseURI = sl

	return c, nil
}

func (nc *Collector) Collect(ctx context.Context, workerID int, tlsConfig *tls.Config, ts *time.Time, concurrent bool) {
	nc.ctx = ctx
	nc.tlsConfig = tlsConfig
	nc.ts = ts
	nc.log = nc.baseLogger.With().Int("worker_id", workerID).Logger()

	collectStart := time.Now()

	baseMeasurementTags := []string{}
	baseStreamTags := []string{"source:kubelet", "node:" + nc.node.Name}

	if nc.verConstraint.Check(nc.kubeletVer) {
		nc.log.Info().Str("kubelet_ver", nc.kubeletVer.String()).Msg("using v2 collector")
		nc.collectV2(concurrent, baseStreamTags, baseMeasurementTags)
	} else {
		nc.log.Info().Str("kubelet_ver", nc.kubeletVer.String()).Msg("using v1 collector")
		nc.collectV1(concurrent, baseStreamTags, baseMeasurementTags)
	}

	nc.check.AddHistSample("collect_latency", cgm.Tags{
		cgm.Tag{Category: "op", Value: "collect_node"},
		cgm.Tag{Category: "source", Value: release.NAME},
		cgm.Tag{Category: "units", Value: "milliseconds"},
	}, float64(time.Since(collectStart).Milliseconds()))
	nc.log.
		Debug().
		Str("duration", time.Since(collectStart).String()).
		Msg("node collect end")
}

// collectV2 is the new collector v1.20+ using only /metrics endpoints
func (nc *Collector) collectV2(concurrent bool, baseStreamTags []string, baseMeasurementTags []string) {
	if concurrent {
		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			nc.meta(baseStreamTags, baseMeasurementTags) // from node list
			wg.Done()
		}()
		if nc.cfg.EnableNodeStats { // this has still not been deprecated... keep pulling until it is
			wg.Add(1)
			go func() {
				nc.summary(baseStreamTags, baseMeasurementTags) // from /stats/summary
				wg.Done()
			}()
		}
		if nc.cfg.EnableNodeMetrics {
			wg.Add(1)
			go func() {
				nc.nmetrics(baseStreamTags, baseMeasurementTags) // from /metrics
				wg.Done()
			}()
		}
		if nc.cfg.EnableNodeResourceMetrics {
			wg.Add(1)
			go func() {
				nc.resources(baseStreamTags, baseMeasurementTags) // from /metrics/resource
				wg.Done()
			}()
		}
		if nc.cfg.EnableNodeProbeMetrics {
			wg.Add(1)
			go func() {
				nc.probes(baseStreamTags, baseMeasurementTags) // from /metrics/probes
				wg.Done()
			}()
		}
		if nc.cfg.EnableCadvisorMetrics {
			wg.Add(1)
			go func() {
				nc.cadvisor(baseStreamTags, baseMeasurementTags) // from /metrics/cadvisor
				wg.Done()
			}()
		}

		wg.Wait()
	} else {
		nc.meta(baseStreamTags, baseMeasurementTags) // from node list
		if nc.cfg.EnableNodeStats {                  // this has still not been deprecated... keep pulling until it is
			nc.summary(baseStreamTags, baseMeasurementTags) // from /stats/summary
		}
		if nc.cfg.EnableNodeMetrics {
			nc.nmetrics(baseStreamTags, baseMeasurementTags) // from /metrics
		}
		if nc.cfg.EnableNodeResourceMetrics {
			nc.resources(baseStreamTags, baseMeasurementTags) // from /metrics/resource
		}
		if nc.cfg.EnableNodeProbeMetrics {
			nc.probes(baseStreamTags, baseMeasurementTags) // from /metrics/probes
		}
		if nc.cfg.EnableCadvisorMetrics {
			nc.cadvisor(baseStreamTags, baseMeasurementTags) // from /metrics/cadvisor
		}
	}
}

// collectV1 is the original collector relying on /stats and cadvisor endpoints
func (nc *Collector) collectV1(concurrent bool, baseStreamTags []string, baseMeasurementTags []string) {
	if concurrent {
		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			nc.meta(baseStreamTags, baseMeasurementTags) // from node list
			wg.Done()
		}()
		if nc.cfg.EnableNodeStats {
			wg.Add(1)
			go func() {
				nc.summary(baseStreamTags, baseMeasurementTags) // from /stats/summary
				wg.Done()
			}()
		}
		if nc.cfg.EnableNodeMetrics {
			wg.Add(1)
			go func() {
				nc.nmetrics(baseStreamTags, baseMeasurementTags) // from /metrics
				wg.Done()
			}()
		}
		if nc.cfg.EnableCadvisorMetrics {
			wg.Add(1)
			go func() {
				nc.cadvisor(baseStreamTags, baseMeasurementTags) // from /metrics/cadvisor
				wg.Done()
			}()
		}

		wg.Wait()
	} else {
		nc.meta(baseStreamTags, baseMeasurementTags) // from node list
		if nc.cfg.EnableNodeStats {
			nc.summary(baseStreamTags, baseMeasurementTags) // from /stats/summary
		}
		if nc.cfg.EnableNodeMetrics {
			nc.nmetrics(baseStreamTags, baseMeasurementTags) // from /metrics
		}
		if nc.cfg.EnableCadvisorMetrics {
			nc.cadvisor(baseStreamTags, baseMeasurementTags) // from /metrics/cadvisor
		}
	}
}

type statsSummary struct {
	Pods []pod            `json:"pods"`
	Node statsSummaryNode `json:"node"`
}

type statsSummaryNode struct {
	NodeName         string      `json:"nodeName"`
	SystemContainers []container `json:"systemContainers"`
	Network          network     `json:"network"`
	Memory           memory      `json:"memory"`
	FS               fs          `json:"fs"`
	RuntimeFS        runtimeFS   `json:"runtime"`
	CPU              cpu         `json:"cpu"`
	Rlimit           rlimit      `json:"rlimit"`
}

type cpu struct {
	UsageNanoCores       uint64 `json:"usageNanoCores"`
	UsageCoreNanoSeconds uint64 `json:"usageCoreNanoSeconds"`
}
type memory struct {
	AvailableBytes  uint64 `json:"availableBytes"`
	UsageBytes      uint64 `json:"usageBytes"`
	WorkingSetBytes uint64 `json:"workingSetBytes"`
	RSSBytes        uint64 `json:"rssBytes"`
	PageFaults      uint64 `json:"pageFaults"`
	MajorPageFaults uint64 `json:"majorPageFaults"`
}
type network struct {
	Interfaces []networkInterface `json:"interfaces"`
	networkInterface
}
type networkInterface struct {
	Name     string `json:"name"`
	RxBytes  uint64 `json:"rxBytes"`
	RxErrors uint64 `json:"rxErrors"`
	TxBytes  uint64 `json:"txBytes"`
	TxErrors uint64 `json:"txErrors"`
}
type fs struct {
	AvailableBytes uint64 `json:"availableBytes"`
	CapacityBytes  uint64 `json:"capacityBytes"`
	UsedBytes      uint64 `json:"usedBytes"`
	InodesFree     uint64 `json:"inodesFree"`
	Inodes         uint64 `json:"inodes"`
	InodesUsed     uint64 `json:"inodesUsed"`
}
type runtimeFS struct {
	ImageFs fs `json:"imageFs"`
}
type rlimit struct {
	MaxPID  uint64 `json:"maxpid"`
	CurProc uint64 `json:"curproc"`
}
type pod struct {
	PodRef           podRef      `json:"podRef"`
	Containers       []container `json:"containers"`
	Volumes          []volume    `json:"volume"`
	Network          network     `json:"network"`
	Memory           memory      `json:"memory"`
	EphemeralStorage fs          `json:"ephemeral-storage"`
	CPU              cpu         `json:"cpu"`
}
type podRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}
type container struct {
	Name   string `json:"name"`
	CPU    cpu    `json:"cpu"`
	Memory memory `json:"memory"`
	RootFS fs     `json:"rootfs"`
	Logs   fs     `json:"logs"`
}
type volume struct {
	Name string `json:"name"`
	fs
}

// resourceMetrics emits pod and container resource requests and limits (if they are set)
func (nc *Collector) resourceMetrics(metrics map[string]circonus.MetricSample, podSpec v1.PodSpec, parentStreamTags []string, parentMeasurementTags []string) {
	lcpu := int64(0)
	lmem := int64(0)
	les := int64(0)
	rcpu := int64(0)
	rmem := int64(0)
	res := int64(0)
	for _, container := range podSpec.Containers {
		ctags := nc.check.NewTagList(parentStreamTags, []string{"container_name:" + container.Name})
		btags := nc.check.NewTagList(ctags, []string{"units:bytes"})
		{
			// limits
			if qty, err := resource.ParseQuantity(container.Resources.Limits.Cpu().String()); err == nil {
				if v := qty.MilliValue(); v > 0 {
					lcpu += v
					if nc.cfg.IncludeContainers {
						mtags := nc.check.NewTagList(ctags, []string{"resource:cpu"})
						_ = nc.check.QueueMetricSample(metrics, "resource_limit", circonus.MetricTypeInt64, mtags, parentMeasurementTags, v, nc.ts)
					}
				}
			}
			if x := container.Resources.Limits.Memory(); x != nil {
				if v, ok := x.AsInt64(); ok && v > 0 {
					lmem += v
					if nc.cfg.IncludeContainers {
						mtags := nc.check.NewTagList(btags, []string{"resource:memory"})
						_ = nc.check.QueueMetricSample(metrics, "resource_limit", circonus.MetricTypeInt64, mtags, parentMeasurementTags, v, nc.ts)
					}
				}
			}
			if x := container.Resources.Limits.StorageEphemeral(); x != nil {
				if v, ok := x.AsInt64(); ok && v > 0 {
					les += v
					if nc.cfg.IncludeContainers {
						mtags := nc.check.NewTagList(btags, []string{"resource:ephemeral_storage"})
						_ = nc.check.QueueMetricSample(metrics, "resource_limit", circonus.MetricTypeInt64, mtags, parentMeasurementTags, v, nc.ts)
					}
				}
			}
		}
		{
			// requests
			if qty, err := resource.ParseQuantity(container.Resources.Requests.Cpu().String()); err == nil {
				if v := qty.MilliValue(); v > 0 {
					rcpu += v
					if nc.cfg.IncludeContainers {
						mtags := nc.check.NewTagList(ctags, []string{"resource:cpu"})
						_ = nc.check.QueueMetricSample(metrics, "resource_request", circonus.MetricTypeInt64, mtags, parentMeasurementTags, v, nc.ts)
					}
				}
			}
			if x := container.Resources.Requests.Memory(); x != nil {
				if v, ok := x.AsInt64(); ok && v > 0 {
					rmem += v
					if nc.cfg.IncludeContainers {
						mtags := nc.check.NewTagList(btags, []string{"resource:memory"})
						_ = nc.check.QueueMetricSample(metrics, "resource_request", circonus.MetricTypeInt64, mtags, parentMeasurementTags, v, nc.ts)
					}
				}
			}
			if x := container.Resources.Requests.StorageEphemeral(); x != nil {
				if v, ok := x.AsInt64(); ok && v > 0 {
					res += v
					if nc.cfg.IncludeContainers {
						mtags := nc.check.NewTagList(btags, []string{"resource:ephemeral_storage"})
						_ = nc.check.QueueMetricSample(metrics, "resource_ephemeral_storage", circonus.MetricTypeInt64, mtags, parentMeasurementTags, v, nc.ts)
					}
				}
			}
		}
	}

	btags := nc.check.NewTagList(parentStreamTags, []string{"units:bytes"})
	// pod limits
	if lcpu > 0 {
		mtags := nc.check.NewTagList(parentStreamTags, []string{"resource:cpu"})
		_ = nc.check.QueueMetricSample(metrics, "resource_limit", circonus.MetricTypeInt64, mtags, parentMeasurementTags, lcpu, nc.ts)
	}
	if lmem > 0 {
		mtags := nc.check.NewTagList(btags, []string{"resource:memory"})
		_ = nc.check.QueueMetricSample(metrics, "resource_limit", circonus.MetricTypeInt64, mtags, parentMeasurementTags, lmem, nc.ts)
	}
	if les > 0 {
		mtags := nc.check.NewTagList(btags, []string{"resource:ephemeral_storage"})
		_ = nc.check.QueueMetricSample(metrics, "resource_limit", circonus.MetricTypeInt64, mtags, parentMeasurementTags, les, nc.ts)
	}
	// pod requests
	if rcpu > 0 {
		mtags := nc.check.NewTagList(parentStreamTags, []string{"resource:cpu"})
		_ = nc.check.QueueMetricSample(metrics, "resource_request", circonus.MetricTypeInt64, mtags, parentMeasurementTags, rcpu, nc.ts)
	}
	if rmem > 0 {
		mtags := nc.check.NewTagList(btags, []string{"resource:memory"})
		_ = nc.check.QueueMetricSample(metrics, "resource_request", circonus.MetricTypeInt64, mtags, parentMeasurementTags, rmem, nc.ts)
	}
	if res > 0 {
		mtags := nc.check.NewTagList(btags, []string{"resource:ephemeral_storage"})
		_ = nc.check.QueueMetricSample(metrics, "resource_request", circonus.MetricTypeInt64, mtags, parentMeasurementTags, res, nc.ts)
	}
}

func (nc *Collector) getPodLabels(pod *v1.Pod) (bool, []string) {
	collect := true
	if nc.cfg.PodLabelKey != "" {
		collect = false
		if v, ok := pod.Labels[nc.cfg.PodLabelKey]; ok {
			if nc.cfg.PodLabelVal == "" {
				collect = true
			} else if v == nc.cfg.PodLabelVal {
				collect = true
			}
		}
	}
	return collect, labelsToTags(pod.Labels)
}

func labelsToTags(labels map[string]string) []string {
	tags := make([]string, len(labels))
	idx := 0
	for k, v := range labels {
		if k == "" {
			continue
		}
		tag := k
		if v != "" {
			tag += ":" + v
		}
		tags[idx] = tag
		idx++
	}
	return tags
}

func (nc *Collector) done() bool {
	select {
	case <-nc.ctx.Done():
		return true
	default:
		return false
	}
}
