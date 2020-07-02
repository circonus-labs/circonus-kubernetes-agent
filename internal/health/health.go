// Copyright © 2020 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package health contains collection/calculation of derived metrics used
// for dashboard and alerting
package health

import (
	"context"
	"crypto/tls"
	"errors"
	"sync"
	"time"

	"github.com/circonus-labs/circonus-kubernetes-agent/internal/circonus"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/release"
	"github.com/rs/zerolog"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Health struct {
	config *config.Cluster
	check  *circonus.Check
	log    zerolog.Logger
}

func New(cfg *config.Cluster, parentLog zerolog.Logger, check *circonus.Check) (*Health, error) {
	if cfg == nil {
		return nil, errors.New("invalid cluster config (nil)")
	}
	if check == nil {
		return nil, errors.New("invalid check (nil)")
	}

	h := &Health{
		config: cfg,
		check:  check,
		log:    parentLog.With().Str("collector", "health").Logger(),
	}
	return h, nil
}

func (h *Health) ID() string {
	return "health"
}

func (h *Health) Collect(ctx context.Context, tlsConfig *tls.Config, ts *time.Time) {
	var cfg *rest.Config
	if c, err := rest.InClusterConfig(); err != nil {
		if err != rest.ErrNotInCluster {
			h.log.Error().Err(err).Msg("unable to initialize health collector")
			return
		}
		// not in cluster, use supplied customer config for cluster
		cfg = &rest.Config{}
		if h.config.BearerToken != "" {
			cfg.BearerToken = h.config.BearerToken
		}
		if h.config.URL != "" {
			cfg.Host = h.config.URL
		}
		if h.config.CAFile != "" {
			cfg.TLSClientConfig = rest.TLSClientConfig{CAFile: h.config.CAFile}
		}
	} else {
		cfg = c // use in-cluster config
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		h.log.Error().Err(err).Msg("initializing client set")
		return
	}

	baseMeasurementTags := []string{}
	baseStreamTags := []string{"source:" + release.NAME}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		h.deployments(ctx, clientset, ts, baseStreamTags, baseMeasurementTags)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		h.daemonsets(ctx, clientset, ts, baseStreamTags, baseMeasurementTags)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		h.statefulsets(ctx, clientset, ts, baseStreamTags, baseMeasurementTags)
		wg.Done()
	}()

	wg.Wait()
}

func (h *Health) deployments(ctx context.Context, cs *kubernetes.Clientset, ts *time.Time, parentStreamTags, parentMeasurementTags []string) {
	metrics := make(map[string]circonus.MetricSample)

	list, err := cs.AppsV1().Deployments("").List(v1.ListOptions{})
	if err != nil {
		h.log.Error().Err(err).Msg("deployments list")
	}

	if len(list.Items) == 0 {
		return // there aren't any deployments in the cluster
	}

	for _, item := range list.Items {
		var streamTags []string
		streamTags = append(streamTags, parentStreamTags...)
		streamTags = append(streamTags, []string{
			"name:" + item.GetName(),
			"namespace:" + item.GetNamespace(),
		}...)
		_ = h.check.QueueMetricSample(
			metrics,
			"deployment_generation_delta",
			circonus.MetricTypeInt64,
			streamTags, parentMeasurementTags,
			item.GetGeneration()-item.Status.ObservedGeneration,
			ts)
	}

	if len(metrics) == 0 {
		h.log.Warn().Msg("no meta deployment health telemetry to submit")
		return
	}

	if err := h.check.SubmitMetrics(ctx, metrics, h.log.With().Str("type", "health-deployment").Logger(), true); err != nil {
		h.log.Warn().Err(err).Msg("submitting metrics")
	}
}

func (h *Health) daemonsets(ctx context.Context, cs *kubernetes.Clientset, ts *time.Time, parentStreamTags, parentMeasurementTags []string) {
	metrics := make(map[string]circonus.MetricSample)

	list, err := cs.AppsV1().DaemonSets("").List(v1.ListOptions{})
	if err != nil {
		h.log.Error().Err(err).Msg("daemonsets list")
	}

	if len(list.Items) == 0 {
		return // there aren't any daemonsets in the cluster
	}

	for _, item := range list.Items {
		var streamTags []string
		streamTags = append(streamTags, parentStreamTags...)
		streamTags = append(streamTags, []string{
			"name:" + item.GetName(),
			"namespace:" + item.GetNamespace(),
		}...)
		_ = h.check.QueueMetricSample(
			metrics,
			"daemonset_scheduled_delta",
			circonus.MetricTypeInt64,
			streamTags, parentMeasurementTags,
			item.Status.DesiredNumberScheduled-item.Status.CurrentNumberScheduled,
			ts)
	}

	if len(metrics) == 0 {
		h.log.Warn().Msg("no meta daemonset health telemetry to submit")
		return
	}

	if err := h.check.SubmitMetrics(ctx, metrics, h.log.With().Str("type", "health-daemonset").Logger(), true); err != nil {
		h.log.Warn().Err(err).Msg("submitting metrics")
	}
}

func (h *Health) statefulsets(ctx context.Context, cs *kubernetes.Clientset, ts *time.Time, parentStreamTags, parentMeasurementTags []string) {
	metrics := make(map[string]circonus.MetricSample)

	list, err := cs.AppsV1().StatefulSets("").List(v1.ListOptions{})
	if err != nil {
		h.log.Error().Err(err).Msg("statefulsets list")
	}

	if len(list.Items) == 0 {
		return // there aren't any statefulsets in the cluster
	}

	for _, item := range list.Items {
		var streamTags []string
		streamTags = append(streamTags, parentStreamTags...)
		streamTags = append(streamTags, []string{
			"name:" + item.GetName(),
			"namespace:" + item.GetNamespace(),
		}...)
		_ = h.check.QueueMetricSample(
			metrics,
			"statefulset_replica_delta",
			circonus.MetricTypeInt64,
			streamTags, parentMeasurementTags,
			*item.Spec.Replicas-item.Status.ReadyReplicas,
			ts)
	}

	if len(metrics) == 0 {
		h.log.Warn().Msg("no meta statefulset health telemetry to submit")
		return
	}

	if err := h.check.SubmitMetrics(ctx, metrics, h.log.With().Str("type", "health-statefulset").Logger(), true); err != nil {
		h.log.Warn().Err(err).Msg("submitting metrics")
	}
}
