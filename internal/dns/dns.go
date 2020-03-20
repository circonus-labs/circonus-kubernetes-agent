// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package dns is the kube-dns collector
package dns

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
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
	"github.com/rs/zerolog"
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
	svc, err := dns.getServiceDefinition(tlsConfig)
	if err != nil {
		dns.log.Error().Err(err).Msg("service definition")
		dns.Lock()
		dns.running = false
		dns.Unlock()
		return
	}
	if svc == nil {
		dns.log.Error().Msg("invalid service definition (nil)")
		dns.Lock()
		dns.running = false
		dns.Unlock()
		return
	}

	metricPath := "/proxy/metrics"
	metricPortName := ""
	for _, p := range svc.Spec.Ports {
		if p.Name == "metrics" {
			metricPortName = p.Name
		}
	}

	if metricPortName == "" {
		dns.log.Error().Msg("invalid service definition, port named 'metrics' not found")
		dns.Lock()
		dns.running = false
		dns.Unlock()
		return
	}

	metricURL := dns.config.URL + svc.Metadata.SelfLink + ":" + metricPortName + metricPath

	if err := dns.metrics(ctx, tlsConfig, metricURL); err != nil {
		dns.log.Error().Err(err).Str("url", metricURL).Msg("http-metrics")
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

func (dns *DNS) getServiceDefinition(tlsConfig *tls.Config) (*k8s.Service, error) {
	u, err := url.Parse(dns.config.URL + "/api/v1/services")
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("fieldSelector", "metadata.name=kube-dns")
	u.RawQuery = q.Encode()

	client, err := k8s.NewAPIClient(tlsConfig, dns.apiTimelimit)
	if err != nil {
		return nil, errors.Wrap(err, "service definition cli")
	}
	defer client.CloseIdleConnections()

	reqURL := u.String()
	dns.log.Debug().Str("url", reqURL).Msg("service")
	req, err := k8s.NewAPIRequest(dns.config.BearerToken, reqURL)
	if err != nil {
		return nil, errors.Wrap(err, "service definition req")
	}

	resp, err := client.Do(req)
	if err != nil {
		dns.check.IncrementCounter("collect_api_errors", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "request", Value: "kube-dns_service"},
			cgm.Tag{Category: "target", Value: "api-server"},
		})
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		dns.check.IncrementCounter("collect_api_errors", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "request", Value: "kube-dns_service"},
			cgm.Tag{Category: "target", Value: "api-server"},
			cgm.Tag{Category: "code", Value: fmt.Sprintf("%d", resp.StatusCode)},
		})
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			dns.log.Error().Err(err).Str("url", reqURL).Msg("reading response")
			return nil, err
		}
		dns.log.Warn().Str("status", resp.Status).RawJSON("response", data).Msg("error from API server")
		return nil, errors.New("error response from api server")
	}

	var s k8s.ServiceList
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return nil, err
	}

	if len(s.Items) == 0 {
		return nil, errors.New("no 'kube-dns' service found")
	}

	if len(s.Items) > 1 {
		return nil, fmt.Errorf("multiple (%d) 'kube-dns' services found", len(s.Items))
	}

	return s.Items[0], nil
}

func (dns *DNS) metrics(ctx context.Context, tlsConfig *tls.Config, metricURL string) error {
	client, err := k8s.NewAPIClient(tlsConfig, dns.apiTimelimit)
	if err != nil {
		return errors.Wrap(err, "/metrics cli")
	}
	defer client.CloseIdleConnections()

	dns.log.Debug().Str("url", metricURL).Msg("metrics")
	req, err := k8s.NewAPIRequest(dns.config.BearerToken, metricURL)
	if err != nil {
		return errors.Wrap(err, "/metrics req")
	}

	start := time.Now()
	resp, err := client.Do(req)
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
		"__rollup:false", // prevent high cardinality metrics from rolling up
	}
	measurementTags := []string{}

	if err := promtext.QueueMetrics(ctx, dns.check, dns.log, resp.Body, streamTags, measurementTags, dns.ts); err != nil {
		return err
	}

	return nil
}
