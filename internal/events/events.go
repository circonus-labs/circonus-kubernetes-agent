// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package events is the cluster events collector
package events

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"time"

	"github.com/circonus-labs/circonus-kubernetes-agent/internal/circonus"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config"
	"github.com/rs/zerolog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type Events struct {
	config *config.Cluster
	check  *circonus.Check
	log    zerolog.Logger
}

func New(cfg *config.Cluster, parentLog zerolog.Logger, check *circonus.Check) (*Events, error) {
	if cfg == nil {
		return nil, errors.New("invalid cluster config (nil)")
	}
	if check == nil {
		return nil, errors.New("invalid check (nil)")
	}

	e := &Events{
		config: cfg,
		check:  check,
		log:    parentLog.With().Str("collector", "events").Logger(),
	}
	return e, nil
}

func (e *Events) ID() string {
	return "events"
}

func (e *Events) Start(ctx context.Context, tlsConfig *tls.Config) {
	e.log.Info().Msg("starting watcher")

	var cfg *rest.Config
	if c, err := rest.InClusterConfig(); err != nil {
		if err != rest.ErrNotInCluster {
			e.log.Error().Err(err).Msg("unable to start event monitor")
			return
		}
		// not in cluster, use supplied customer config for cluster
		cfg = &rest.Config{}
		if e.config.BearerToken != "" {
			cfg.BearerToken = e.config.BearerToken
		}
		if e.config.URL != "" {
			cfg.Host = e.config.URL
		}
		if e.config.CAFile != "" {
			cfg.TLSClientConfig = rest.TLSClientConfig{CAFile: e.config.CAFile}
		}
	} else {
		cfg = c // use in-cluster config
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		e.log.Error().Err(err).Msg("initializing client set")
		return
	}

	factory := informers.NewSharedInformerFactory(clientset, 0)
	informer := factory.Core().V1().Events().Informer()
	stopper := make(chan struct{})
	defer close(stopper)
	defer runtime.HandleCrash()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			e.submitEvent(ctx, obj.(*corev1.Event))
		},
		// UpdateFunc: func(oldObj interface{}, newObj interface{}) {
		// 	e.submitEvent(newObj.(*corev1.Event))
		// },
	})

	go informer.Run(stopper)

	if !cache.WaitForCacheSync(stopper, informer.HasSynced) {
		e.log.Warn().Msg("timed out waiting for cache to sync")
		return
	}

	go func() {
		var streamTags []string
		var measurementTags []string
		ets := time.Now()
		ae := abridgedEvent{
			CreationTimestamp: ets.UTC().Unix(),
			Reason:            "enabled",
			Message:           "enabled",
		}

		data, err := json.Marshal(ae)
		if err != nil {
			e.log.Error().Err(err).Str("data", string(data)).Msg("parsing 'initial' event")
			return
		}

		metrics := make(map[string]circonus.MetricSample)
		_ = e.check.QueueMetricSample(
			metrics,
			"events",
			circonus.MetricTypeString,
			streamTags, measurementTags,
			string(data),
			&ets)
		if err := e.check.SubmitMetrics(ctx, metrics, e.log.With().Str("type", "event").Logger(), true); err != nil {
			e.log.Warn().Err(err).Msg("submitting initial event")
		}
	}()

	<-ctx.Done()
	e.log.Debug().Msg("closing event watcher")
}

type abridgedEvent struct {
	Namespace         string `json:"namespace"`
	SelfLink          string `json:"selfLink"`
	CreationTimestamp int64  `json:"creationTimestamp"`
	Reason            string `json:"reason"`
	Message           string `json:"message"`
}

func (e *Events) submitEvent(ctx context.Context, event *corev1.Event) {
	ets := event.GetCreationTimestamp().UTC()
	ae := abridgedEvent{
		Namespace:         event.GetNamespace(),
		SelfLink:          event.GetSelfLink(),
		CreationTimestamp: event.GetCreationTimestamp().UTC().Unix(),
		Reason:            event.Reason,
		Message:           event.Message,
	}

	data, err := json.Marshal(ae)
	if err != nil {
		e.log.Error().Err(err).Str("data", string(data)).Msg("parsing event")
		return
	}

	var streamTags []string
	var measurementTags []string
	metrics := make(map[string]circonus.MetricSample)
	_ = e.check.QueueMetricSample(
		metrics,
		"events",
		circonus.MetricTypeString,
		streamTags, measurementTags,
		string(data),
		&ets)

	if err := e.check.SubmitMetrics(ctx, metrics, e.log.With().Str("type", "event").Logger(), true); err != nil {
		e.log.Warn().Err(err).Msg("submitting event")
	}
}
