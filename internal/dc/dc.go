// Copyright © 2020 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package dc is the dynamic collector
package dc

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/circonus"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/k8s"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/promtext"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/release"
	"github.com/prometheus/common/expfmt"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DC struct {
	config     *config.Cluster
	check      *circonus.Check
	collectors []Collector `yaml:"collectors"`
	log        zerolog.Logger
	running    bool
	sync.Mutex
	ts *time.Time
}

type Collectors struct {
	Collectors []Collector `yaml:"collectors"`
}

type Collector struct {
	Name       string     `yaml:"name"`
	Disable    bool       `yaml:"disable"`
	Type       string     `yaml:"type"`
	Schema     string     `yaml:"schema"`
	Selectors  Selectors  `yaml:"selectors"`
	Control    Control    `yaml:"control"`
	MetricPort MetricPort `yaml:"metric_port"`
	MetricPath MetricPath `yaml:"metric_path"`
	Tags       string     `yaml:"tags"`
	LabelTags  string     `yaml:"label_tags"`
	Rollup     bool       `yaml:"rollup"`
}

type Selectors struct {
	Label string `yaml:"label"`
	Field string `yaml:"field"`
}

type Control struct {
	Annotation string `yaml:"annotation"`
	Label      string `yaml:"label"`
	Value      string `yaml:"value"`
}

type MetricPort struct {
	Annotation string `yaml:"annotation"`
	Label      string `yaml:"label"`
	Value      string `yaml:"value"`
}

type MetricPath struct {
	Annotation string `yaml:"annotation"`
	Label      string `yaml:"label"`
	Value      string `yaml:"value"`
}

func New(cfg *config.Cluster, parentLogger zerolog.Logger, check *circonus.Check) (*DC, error) {
	dc := &DC{
		config: cfg,
		check:  check,
		log:    parentLogger.With().Str("pkg", "dynamic-collectors").Logger(),
	}

	configFile := cfg.DynamicCollectorFile
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	var cc Collectors
	if err := yaml.Unmarshal(data, &cc); err != nil {
		return nil, fmt.Errorf("unable to parse dynamic collectors config (%s): %w", configFile, err)
	}

	dc.collectors = make([]Collector, 0)
	for idx, collector := range cc.Collectors {
		if collector.Disable {
			continue
		}
		if collector.Name == "" {
			dc.log.Warn().Int("position", idx).Msg("invalid collector, 'name' missing, skipping")
			continue
		}
		if collector.MetricPath.Value == "" && collector.MetricPath.Annotation == "" && collector.MetricPath.Label == "" {
			collector.MetricPath.Value = "/metrics"
		}
		if collector.Schema == "" {
			collector.Schema = "http"
		}
		dc.collectors = append(dc.collectors, collector)
	}

	if len(dc.collectors) == 0 {
		return nil, fmt.Errorf("invalid dynamic collectors config (%s) zero collectors defined", configFile)
	}

	return dc, nil
}

func (dc *DC) Collect(ctx context.Context, tlsConfig *tls.Config, ts *time.Time) {
	dc.Lock()
	if dc.running {
		dc.log.Warn().Msg("already running")
		dc.Unlock()
		return
	}
	dc.running = true
	dc.ts = ts
	dc.Unlock()

	defer func() {
		if r := recover(); r != nil {
			dc.log.Error().Interface("panic", r).Msg("recover")
			dc.Lock()
			dc.running = false
			dc.Unlock()
		}
	}()

	collectStart := time.Now()
	var wg sync.WaitGroup

	for _, collector := range dc.collectors {
		switch strings.ToLower(collector.Type) {
		case "endpoints":
			wg.Add(1)
			go func(collector Collector) {
				dc.collectEndpoints(ctx, collector)
				wg.Done()
			}(collector)
		case "nodes":
			wg.Add(1)
			go func(collector Collector) {
				dc.collectNodes(ctx, collector)
				wg.Done()
			}(collector)
		case "pods":
			wg.Add(1)
			go func(collector Collector) {
				dc.collectPods(ctx, collector)
				wg.Done()
			}(collector)
		case "services":
			wg.Add(1)
			go func(collector Collector) {
				dc.collectServices(ctx, collector)
				wg.Done()
			}(collector)
		default:
			dc.log.Warn().Str("name", collector.Name).Str("type", collector.Type).Msg("unknown/unsupported collector type, skipping")
		}
	}

	wg.Wait()

	dc.check.AddHistSample("collect_latency", cgm.Tags{
		cgm.Tag{Category: "source", Value: release.NAME},
		cgm.Tag{Category: "op", Value: "collect_dynamic-collectors"},
		cgm.Tag{Category: "units", Value: "milliseconds"},
	}, float64(time.Since(collectStart).Milliseconds()))

	dc.log.Debug().Str("duration", time.Since(collectStart).String()).Msg("dynamic-collectors collect end")
	dc.Lock()
	dc.running = false
	dc.Unlock()
}

type metricTarget struct {
	URL  string
	Tags []string
}

func (dc *DC) collectEndpoints(ctx context.Context, collector Collector) {
	logger := dc.log.With().Str("collector-type", collector.Type).Str("collector-name", collector.Name).Logger()

	clientset, err := k8s.GetClient(dc.config)
	if err != nil {
		logger.Warn().Err(err).Msg("initializing k8s client")
		return
	}

	opts := metav1.ListOptions{}
	if collector.Selectors.Field != "" {
		opts.FieldSelector = collector.Selectors.Field
	}
	if collector.Selectors.Label != "" {
		opts.LabelSelector = collector.Selectors.Label
	}

	endpoints, err := clientset.CoreV1().Endpoints("").List(opts)
	if err != nil {
		logger.Warn().Err(err).Msg("querying k8s endpoints")
		return
	}

	targets := make([]metricTarget, 0)
	for _, item := range endpoints.Items {
		if !dc.collectItem(collector, item.Labels, item.Annotations) {
			continue
		}
		port := dc.getPort(collector, item.Labels, item.Annotations)
		if port == "" {
			logger.Warn().Str("endpoint", item.Name).Interface("collector", collector).Msg("unable to find metric port, skipping")
			continue
		}
		path := dc.getPath(collector, item.Labels, item.Annotations)
		if path == "" {
			logger.Warn().Str("endpoint", item.Name).Interface("collector", collector).Msg("unable to find metric path, skipping")
			continue
		}

		for _, subset := range item.Subsets {
			for _, addr := range subset.Addresses {
				u := url.URL{
					Scheme: collector.Schema,
					Host:   addr.IP + ":" + port,
					Path:   path,
				}
				tags := generateTags(collector.Tags, collector.LabelTags, item.Labels)
				tags = append(tags, "collector_target:"+addr.TargetRef.Name)
				targets = append(targets, metricTarget{URL: u.String(), Tags: tags})
			}
		}

		if done(ctx) {
			return
		}
	}

	for _, target := range targets {
		dc.getMetrics(ctx, collector, target, logger)
		if done(ctx) {
			return
		}
	}
}

func (dc *DC) collectNodes(ctx context.Context, collector Collector) {
	logger := dc.log.With().Str("collector-type", collector.Type).Str("collector-name", collector.Name).Logger()

	clientset, err := k8s.GetClient(dc.config)
	if err != nil {
		logger.Warn().Err(err).Msg("initializing k8s client")
		return
	}

	opts := metav1.ListOptions{}
	if collector.Selectors.Field != "" {
		opts.FieldSelector = collector.Selectors.Field
	}
	if collector.Selectors.Label != "" {
		opts.LabelSelector = collector.Selectors.Label
	}

	nodes, err := clientset.CoreV1().Nodes().List(opts)
	if err != nil {
		logger.Warn().Err(err).Msg("querying k8s nodes")
		return
	}

	targets := make([]metricTarget, 0)
	for _, item := range nodes.Items {
		if !dc.collectItem(collector, item.Labels, item.Annotations) {
			continue
		}
		port := dc.getPort(collector, item.Labels, item.Annotations)
		if port == "" {
			logger.Warn().Str("node", item.Name).Interface("collector", collector).Msg("unable to find metric port, skipping")
			continue
		}
		path := dc.getPath(collector, item.Labels, item.Annotations)
		if path == "" {
			logger.Warn().Str("node", item.Name).Interface("collector", collector).Msg("unable to find metric path, skipping")
			continue
		}

		ip := ""
		for _, addr := range item.Status.Addresses {
			if addr.Type == v1.NodeInternalIP {
				ip = addr.Address
			}
		}
		if ip == "" {
			logger.Warn().Str("node", item.Name).Msg("no internal IP found, skipping")
			continue
		}

		u := url.URL{
			Scheme: collector.Schema,
			Host:   ip + ":" + port,
			Path:   path,
		}
		tags := generateTags(collector.Tags, collector.LabelTags, item.Labels)
		tags = append(tags, "collector_target:"+item.Name)
		targets = append(targets, metricTarget{URL: u.String(), Tags: tags})

		if done(ctx) {
			return
		}
	}

	for _, target := range targets {
		dc.getMetrics(ctx, collector, target, logger)
		if done(ctx) {
			return
		}
	}
}

func (dc *DC) collectPods(ctx context.Context, collector Collector) {
	logger := dc.log.With().Str("collector-type", collector.Type).Str("collector-name", collector.Name).Logger()

	clientset, err := k8s.GetClient(dc.config)
	if err != nil {
		logger.Warn().Err(err).Msg("initializing k8s client")
		return
	}

	opts := metav1.ListOptions{}
	if collector.Selectors.Field != "" {
		opts.FieldSelector = collector.Selectors.Field
	}
	if collector.Selectors.Label != "" {
		opts.LabelSelector = collector.Selectors.Label
	}

	pods, err := clientset.CoreV1().Pods("").List(opts)
	if err != nil {
		logger.Warn().Err(err).Msg("querying k8s pods")
		return
	}

	targets := make([]metricTarget, 0)
	for _, item := range pods.Items {
		if !dc.collectItem(collector, item.Labels, item.Annotations) {
			continue
		}
		port := dc.getPort(collector, item.Labels, item.Annotations)
		if port == "" {
			logger.Warn().Str("pod", item.Name).Interface("collector", collector).Msg("unable to find metric port, skipping")
			continue
		}
		path := dc.getPath(collector, item.Labels, item.Annotations)
		if path == "" {
			logger.Warn().Str("pod", item.Name).Interface("collector", collector).Msg("unable to find metric path, skipping")
			continue
		}

		ip := item.Status.PodIP
		if ip == "" {
			logger.Warn().Str("pod", item.Name).Msg("no Pod IP found, skipping")
			continue
		}

		u := url.URL{
			Scheme: collector.Schema,
			Host:   ip + ":" + port,
			Path:   path,
		}
		tags := generateTags(collector.Tags, collector.LabelTags, item.Labels)
		tags = append(tags, "collector_target:"+item.Name)
		targets = append(targets, metricTarget{URL: u.String(), Tags: tags})

		if done(ctx) {
			return
		}
	}

	for _, target := range targets {
		dc.getMetrics(ctx, collector, target, logger)
		if done(ctx) {
			return
		}
	}
}

func (dc *DC) collectServices(ctx context.Context, collector Collector) {
	logger := dc.log.With().Str("collector-type", collector.Type).Str("collector-name", collector.Name).Logger()

	clientset, err := k8s.GetClient(dc.config)
	if err != nil {
		logger.Warn().Err(err).Msg("initializing k8s client")
		return
	}

	opts := metav1.ListOptions{}
	if collector.Selectors.Field != "" {
		opts.FieldSelector = collector.Selectors.Field
	}
	if collector.Selectors.Label != "" {
		opts.LabelSelector = collector.Selectors.Label
	}

	services, err := clientset.CoreV1().Services("").List(opts)
	if err != nil {
		logger.Warn().Err(err).Msg("querying k8s services")
		return
	}

	targets := make([]metricTarget, 0)
	for _, item := range services.Items {
		if !dc.collectItem(collector, item.Labels, item.Annotations) {
			continue
		}
		port := dc.getPort(collector, item.Labels, item.Annotations)
		if port == "" {
			logger.Warn().Str("service", item.Name).Interface("collector", collector).Msg("unable to find metric port, skipping")
			continue
		}
		path := dc.getPath(collector, item.Labels, item.Annotations)
		if path == "" {
			logger.Warn().Str("service", item.Name).Interface("collector", collector).Msg("unable to find metric path, skipping")
			continue
		}

		ip := item.Spec.ClusterIP
		if ip == "" || ip == v1.ClusterIPNone {
			logger.Warn().Str("service", item.Name).Msg("no Cluster IP found, skipping")
			continue
		}

		u := url.URL{
			Scheme: collector.Schema,
			Host:   ip + ":" + port,
			Path:   path,
		}
		tags := generateTags(collector.Tags, collector.LabelTags, item.Labels)
		tags = append(tags, "collector_target:"+item.Name)
		targets = append(targets, metricTarget{URL: u.String(), Tags: tags})

		if done(ctx) {
			return
		}
	}

	for _, target := range targets {
		dc.getMetrics(ctx, collector, target, logger)
		if done(ctx) {
			return
		}
	}
}

// getMetrics fetches the metrics from a url, parses them and submits them to circonus
func (dc *DC) getMetrics(ctx context.Context, collector Collector, target metricTarget, logger zerolog.Logger) {
	if done(ctx) {
		return
	}

	var data *bytes.Reader

	start := time.Now()

	logger.Debug().Str("url", target.URL).Msg("getting metrics")

	client := &http.Client{}
	if strings.HasPrefix(target.URL, "https:") {
		client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec
	}
	req, err := http.NewRequestWithContext(ctx, "GET", target.URL, nil)
	if err != nil {
		logger.Warn().Err(err).Str("url", target.URL).Msg("creating request")
		return
	}
	req.Header.Add("User-Agent", release.NAME+"/"+release.VERSION)
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	if err != nil {
		dc.check.IncrementCounter("collect_dc_errors", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "dcn", Value: collector.Name},
			cgm.Tag{Category: "request", Value: target.URL},
		})
		logger.Warn().Err(err).Str("url", target.URL).Msg("making request")
		return
	}
	defer resp.Body.Close()
	dc.check.AddHistSample("collect_latency", cgm.Tags{
		cgm.Tag{Category: "source", Value: release.NAME},
		cgm.Tag{Category: "dcn", Value: collector.Name},
		cgm.Tag{Category: "request", Value: target.URL},
		cgm.Tag{Category: "units", Value: "milliseconds"},
	}, float64(time.Since(start).Milliseconds()))

	d, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Error().Err(err).Str("url", target.URL).Msg("reading response")
		return
	}
	if resp.StatusCode != http.StatusOK {
		dc.check.IncrementCounter("collect_dc_errors", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "dcn", Value: collector.Name},
			cgm.Tag{Category: "request", Value: target.URL},
			cgm.Tag{Category: "code", Value: fmt.Sprintf("%d", resp.StatusCode)},
		})
		logger.Warn().Str("status", resp.Status).RawJSON("response", d).Str("url", target.URL).Msg("error from target")
		return
	}
	data = bytes.NewReader(d)
	streamTags := []string{
		"collector:dynamic",
		"collector_name:" + collector.Name,
		"collector_type:" + collector.Type,
	}
	if !collector.Rollup {
		streamTags = append(streamTags, "__rollup:false") // prevent high cardinality metrics from rolling up
	}
	streamTags = append(streamTags, target.Tags...)
	measurementTags := []string{}

	var parser expfmt.TextParser
	if err := promtext.QueueMetrics(ctx, parser, dc.check, logger, data, streamTags, measurementTags, dc.ts); err != nil {
		logger.Warn().Err(err).Str("url", target.URL).Msg("parsing metrics")
		return
	}
}

// collectItem uses the configuration's Control settings to determine if the specific item should be collected
func (dc *DC) collectItem(collector Collector, labels map[string]string, annotations map[string]string) bool {
	// no annotation or label
	if collector.Control.Annotation == "" && collector.Control.Label == "" {
		return true
	}

	// no value to compare the annotation/label against
	if collector.Control.Value == "" {
		return true
	}

	if collector.Control.Annotation != "" {
		for an, av := range annotations {
			if an == collector.Control.Annotation {
				return av == collector.Control.Value
			}
		}
	}

	if collector.Control.Label != "" {
		for ln, lv := range labels {
			if ln == collector.Control.Label {
				return lv == collector.Control.Value
			}
		}
	}

	return false
}

// getPort uses the configuration's MetricPort settings to determine what port to use for metric request
func (dc *DC) getPort(collector Collector, labels map[string]string, annotations map[string]string) string {
	if collector.MetricPort.Value != "" {
		return collector.MetricPort.Value
	}

	if collector.MetricPort.Annotation != "" {
		for an, av := range annotations {
			if an == collector.MetricPort.Annotation {
				return av
			}
		}
	}

	if collector.MetricPort.Label != "" {
		for ln, lv := range labels {
			if ln == collector.MetricPort.Label {
				return lv
			}
		}
	}

	return ""
}

// getPath uses the configuration's MetricPath settings to determine what path to use for metric request
func (dc *DC) getPath(collector Collector, labels map[string]string, annotations map[string]string) string {
	if collector.MetricPath.Value != "" {
		return collector.MetricPath.Value
	}

	if collector.MetricPath.Annotation != "" {
		for an, av := range annotations {
			if an == collector.MetricPath.Annotation {
				return av
			}
		}
	}

	if collector.MetricPath.Label != "" {
		for ln, lv := range labels {
			if ln == collector.MetricPath.Label {
				return lv
			}
		}
	}

	return ""
}

// generateTags creates the initial streamtags for the metric based on configured tags and labels
func generateTags(tags string, labels string, itemLabels map[string]string) []string {
	tagList := make([]string, 0)
	if tags != "" {
		tt := strings.Split(tags, ",")
		for _, t := range tt {
			tagList = append(tagList, strings.TrimSpace(t))
		}
	}
	if labels != "" {
		ll := strings.Split(labels, ",")
		for ln, lv := range itemLabels {
			for _, l := range ll {
				if strings.TrimSpace(l) == ln {
					tagList = append(tagList, ln+":"+lv)
					break
				}
			}
		}
	}
	return tagList
}

func done(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}
