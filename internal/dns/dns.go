// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package dns is the kube-dns collector
package dns

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/circonus"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config/defaults"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/promtext"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/release"
	"github.com/pkg/errors"
	"github.com/prometheus/common/expfmt"
	"github.com/rs/zerolog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type DNS struct {
	config       *config.Cluster
	check        *circonus.Check
	log          zerolog.Logger
	running      bool
	apiTimelimit time.Duration
	sync.Mutex
	ts *time.Time
}

func New(cfg *config.Cluster, parentLog zerolog.Logger, check *circonus.Check) (*DNS, error) {
	if cfg == nil {
		return nil, errors.New("invalid cluster config (nil)")
	}
	if check == nil {
		return nil, errors.New("invalid check (nil)")
	}

	dns := &DNS{
		config: cfg,
		check:  check,
		log:    parentLog.With().Str("collector", "kube-dns").Logger(),
	}

	if cfg.APITimelimit != "" {
		v, err := time.ParseDuration(cfg.APITimelimit)
		if err != nil {
			dns.log.Error().Err(err).Msg("parsing api timelimit, using default")
		} else {
			dns.apiTimelimit = v
		}
	}

	if dns.apiTimelimit == time.Duration(0) {
		v, err := time.ParseDuration(defaults.K8SAPITimelimit)
		if err != nil {
			dns.log.Fatal().Err(err).Msg("parsing DEFAULT api timelimit")
		}
		dns.apiTimelimit = v
	}

	return dns, nil
}

func (dns *DNS) ID() string {
	return "kube-dns"
}

func (dns *DNS) Collect(ctx context.Context, tlsConfig *tls.Config, ts *time.Time) {
	dns.Lock()
	if dns.running {
		dns.log.Warn().Msg("already running")
		dns.Unlock()
		return
	}
	dns.running = true
	dns.ts = ts
	dns.Unlock()

	defer func() {
		if r := recover(); r != nil {
			dns.log.Error().Interface("panic", r).Msg("recover")
			dns.Lock()
			dns.running = false
			dns.Unlock()
		}
	}()

	collectStart := time.Now()

	urls, err := dns.getMetricURLs()
	if err != nil {
		dns.log.Error().Err(err).Msg("invalid service definition")
		dns.Lock()
		dns.running = false
		dns.Unlock()
		return
	}

	for podName, metricURL := range urls {
		if err := dns.getMetrics(ctx, podName, metricURL); err != nil {
			dns.log.Error().Err(err).Str("url", metricURL).Msg("http-metrics")
		}
	}

	dns.check.AddHistSample("collect_latency", cgm.Tags{
		cgm.Tag{Category: "source", Value: release.NAME},
		cgm.Tag{Category: "opt", Value: "collect_kube-dns"},
		cgm.Tag{Category: "units", Value: "milliseconds"},
	}, float64(time.Since(collectStart).Milliseconds()))
	dns.log.Debug().Str("duration", time.Since(collectStart).String()).Msg("kube-dns collect end")
	dns.Lock()
	dns.running = false
	dns.Unlock()
}

func (dns *DNS) getMetricURLs() (map[string]string, error) {
	var cfg *rest.Config
	if c, err := rest.InClusterConfig(); err != nil {
		if err != rest.ErrNotInCluster {
			return nil, errors.Wrap(err, "unable to get DNS metrics, must be in cluster")
		}
		// not in cluster, use supplied customer config for cluster
		cfg = &rest.Config{}
		if dns.config.BearerToken != "" {
			cfg.BearerToken = dns.config.BearerToken
		}
		if dns.config.URL != "" {
			cfg.Host = dns.config.URL
		}
		if dns.config.CAFile != "" {
			cfg.TLSClientConfig = rest.TLSClientConfig{CAFile: dns.config.CAFile}
		}
	} else {
		cfg = c // use in-cluster config
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "initializing cleint set")
	}

	svc, err := clientset.CoreV1().Services("kube-system").Get("kube-dns", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	scrape := false
	port := ""

	for name, value := range svc.Annotations {
		switch name {
		case "prometheus.io/port":
			port = value
		case "prometheus.io/scrape":
			s, err := strconv.ParseBool(value)
			if err != nil {
				return nil, errors.Wrap(err, "parsing service confing annotation")
			}
			scrape = s
		}
	}
	if !scrape {
		return nil, errors.New("kube-dns not configured for scraping")
	}

	if len(svc.Spec.Selector) == 0 {
		return nil, errors.New("no selectors found in kube-dns service")

	}

	selectors := make([]string, len(svc.Spec.Selector))
	i := 0
	for name, value := range svc.Spec.Selector {
		selectors[i] = name + "=" + value
		i++
	}

	pods, err := clientset.CoreV1().Pods(svc.Namespace).List(metav1.ListOptions{LabelSelector: strings.Join(selectors, ",")})
	if err != nil {
		return nil, errors.Wrap(err, "getting list of kube-dns pods")
	}
	if len(pods.Items) == 0 {
		return nil, errors.Errorf("no pods found matching selector (%s)", strings.Join(selectors, ","))
	}

	urls := make(map[string]string)
	for _, pod := range pods.Items {
		if pod.Status.PodIP != "" {
			urls[pod.Name] = fmt.Sprintf("http://%s:%s/metrics", pod.Status.PodIP, port)
		}
	}

	return urls, nil
}

func (dns *DNS) getMetrics(ctx context.Context, podName, metricURL string) error {

	start := time.Now()
	resp, err := http.Get(metricURL) //nolint:gosec
	if err != nil {
		dns.check.IncrementCounter("collect_api_errors", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "request", Value: "metrics"},
			cgm.Tag{Category: "proxy", Value: "api-server"},
			cgm.Tag{Category: "target", Value: "kube-dns"},
		})
		return err
	}
	defer resp.Body.Close()
	dns.check.AddHistSample("collect_latency", cgm.Tags{
		cgm.Tag{Category: "source", Value: release.NAME},
		cgm.Tag{Category: "request", Value: "metrics"},
		cgm.Tag{Category: "proxy", Value: "api-server"},
		cgm.Tag{Category: "target", Value: "kube-dns"},
		cgm.Tag{Category: "units", Value: "milliseconds"},
	}, float64(time.Since(start).Milliseconds()))

	if resp.StatusCode != http.StatusOK {
		dns.check.IncrementCounter("collect_api_errors", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "request", Value: "metrics"},
			cgm.Tag{Category: "proxy", Value: "api-server"},
			cgm.Tag{Category: "target", Value: "kube-dns"},
			cgm.Tag{Category: "code", Value: fmt.Sprintf("%d", resp.StatusCode)},
		})
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			dns.log.Error().Err(err).Str("url", metricURL).Msg("reading response")
			return err
		}
		dns.log.Warn().Str("status", resp.Status).RawJSON("response", data).Msg("error from API server")
		return errors.New("error response from api server")
	}

	streamTags := []string{
		"source:kube-dns",
		"source_type:metrics",
		"pod_name:" + podName,
		"__rollup:false", // prevent high cardinality metrics from rolling up
	}
	measurementTags := []string{}

	var parser expfmt.TextParser
	if err := promtext.QueueMetrics(ctx, parser, dns.check, dns.log, resp.Body, streamTags, measurementTags, dns.ts); err != nil {
		return err
	}

	return nil
}
