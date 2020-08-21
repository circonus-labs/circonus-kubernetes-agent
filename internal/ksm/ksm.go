// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package ksm is the kube-state-metrics collector
package ksm

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/circonus"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config/defaults"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/k8s"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/release"
	"github.com/pkg/errors"
	"github.com/prometheus/common/expfmt"
	"github.com/rs/zerolog"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type KSM struct {
	config       *config.Cluster
	check        *circonus.Check
	log          zerolog.Logger
	apiTimelimit time.Duration
	running      bool
	cgmMetrics   *cgm.CirconusMetrics
	sync.Mutex
	ts *time.Time
}

const (
	modeProxy  = "proxy"
	modeDirect = "direct"
)

// NOTES:
// curl -v localhost:8080/api/v1/services?fieldSelector=metadata.name%3Dkube-state-metrics
// for "proxy" mode:
//   the spec.ports.name
//   combine selfLink with ':http-metrics/proxy/metrics' for metrics
//   combine selfLink with ':telemetry/proxy/metrics' for ksm telemetry
// for "direct" mode:
//   use service endpoint w/ports (configured for each port name)

func New(cfg *config.Cluster, parentLogger zerolog.Logger, check *circonus.Check) (*KSM, error) {
	if cfg == nil {
		return nil, errors.New("invalid cluster config (nil)")
	}
	if check == nil {
		return nil, errors.New("invalid check (nil)")
	}

	ksm := &KSM{
		config: cfg,
		check:  check,
		log:    parentLogger.With().Str("collector", "kube-state-metrics").Logger(),
	}

	if cfg.APITimelimit != "" {
		v, err := time.ParseDuration(cfg.APITimelimit)
		if err != nil {
			ksm.log.Error().Err(err).Msg("parsing api timelimit, using default")
		} else {
			ksm.apiTimelimit = v
		}
	}

	if ksm.apiTimelimit == time.Duration(0) {
		v, err := time.ParseDuration(defaults.K8SAPITimelimit)
		if err != nil {
			ksm.log.Fatal().Err(err).Msg("parsing DEFAULT api timelimit")
		}
		ksm.apiTimelimit = v
	}

	{
		// create a cgm container
		cmc := &cgm.Config{
			Debug: false,
			Log:   nil,
		}
		// put cgm into manual mode (no interval, no api key, invalid submission url)
		cmc.Interval = "0"                            // disable automatic flush
		cmc.CheckManager.Check.SubmissionURL = "none" // disable check management (create/update)

		hm, err := cgm.NewCirconusMetrics(cmc)
		if err != nil {
			return nil, errors.Wrap(err, "receiver cgm")
		}

		ksm.cgmMetrics = hm

	}

	return ksm, nil
}

func (ksm *KSM) ID() string {
	return "kube-state-metrics"
}

// Collect metrics from kube-state-metrics
func (ksm *KSM) Collect(ctx context.Context, tlsConfig *tls.Config, ts *time.Time) {
	ksm.Lock()
	if ksm.running {
		ksm.log.Warn().Msg("already running")
		ksm.Unlock()
		return
	}
	ksm.running = true
	ksm.ts = ts
	ksm.Unlock()

	defer func() {
		if r := recover(); r != nil {
			ksm.log.Error().Interface("panic", r).Msg("recover")
			ksm.Lock()
			ksm.running = false
			ksm.Unlock()
		}
	}()

	collectStart := time.Now()
	svc, err := ksm.getServiceDefinition()
	if err != nil {
		ksm.check.AddText("collect_ksm_state", cgm.Tags{
			cgm.Tag{Category: "cluster", Value: ksm.config.Name},
			cgm.Tag{Category: "source", Value: release.NAME},
		}, err.Error())
		ksm.log.Error().Err(err).Msg("service definition")
		ksm.Lock()
		ksm.running = false
		ksm.Unlock()
		return
	}
	if svc == nil {
		ksm.check.AddText("collect_ksm_state", cgm.Tags{
			cgm.Tag{Category: "cluster", Value: ksm.config.Name},
			cgm.Tag{Category: "source", Value: release.NAME},
		}, "invalid service definition")
		ksm.log.Error().Msg("invalid service definition (nil)")
		ksm.Lock()
		ksm.running = false
		ksm.Unlock()
		return
	}

	metricPortName := ""
	telemetryPortName := ""
	for _, p := range svc.Spec.Ports {
		if p.Name != "" {
			if ksm.config.KSMMetricsPortName != "" && ksm.config.KSMMetricsPortName == p.Name {
				metricPortName = p.Name
			} else if ksm.config.KSMTelemetryPortName != "" && ksm.config.KSMTelemetryPortName == p.Name {
				telemetryPortName = p.Name
			}
		}
	}

	if metricPortName == "" && telemetryPortName == "" {
		ksm.check.AddText("collect_ksm_state", cgm.Tags{
			cgm.Tag{Category: "cluster", Value: ksm.config.Name},
			cgm.Tag{Category: "source", Value: release.NAME},
		}, "invalid service definition, named ports not found")
		ksm.log.Error().
			Str("metrics_port", ksm.config.KSMMetricsPortName).
			Str("telemetry_port", ksm.config.KSMTelemetryPortName).
			Msg("invalid service definition, named ports not found")
		ksm.Lock()
		ksm.running = false
		ksm.Unlock()
		return
	}

	if ksm.config.KSMMetricsPortName != "" && metricPortName == "" {
		ksm.log.Warn().Str("port", ksm.config.KSMMetricsPortName).Msg("metrics port not found in service definition")
	}
	if ksm.config.KSMTelemetryPortName != "" && telemetryPortName == "" {
		ksm.log.Warn().Str("port", ksm.config.KSMTelemetryPortName).Msg("telemetry port not found in service definition")
	}

	var wg sync.WaitGroup

	collected := 0
	collectErr := 0

	switch ksm.config.KSMRequestMode {
	case modeProxy:
		metricPath := "/proxy/metrics"

		// NOTE: w/re to ksm being run with HTTPS - https://kubernetes.io/docs/tasks/access-application-cluster/access-cluster/#manually-constructing-apiserver-proxy-urls
		//       if the port name is prefixed with 'https-', "https:" will be added before the service name in the selfLink below.

		if metricPortName != "" {
			wg.Add(1)
			go func() {
				svcPath := svc.SelfLink
				if strings.HasPrefix(metricPortName, "https-") {
					svcPath = strings.Replace(svcPath, svc.Name, "https:"+svc.Name, -1)
				}

				metricURL := svcPath + ":" + metricPortName + metricPath
				if err := ksm.metrics(ctx, metricURL); err != nil {
					ksm.log.Error().Err(err).Str("url", metricURL).Msg("http-metrics")
					collectErr++
				}
				collected++
				wg.Done()
			}()
		}

		if telemetryPortName != "" {
			wg.Add(1)
			go func() {
				svcPath := svc.SelfLink
				if strings.HasPrefix(telemetryPortName, "https-") {
					svcPath = strings.Replace(svcPath, svc.Name, "https:"+svc.Name, -1)
				}

				telemetryURL := svcPath + ":" + telemetryPortName + metricPath
				if err := ksm.telemetry(ctx, telemetryURL); err != nil {
					ksm.log.Error().Err(err).Str("url", telemetryURL).Msg("telemetry")
					collectErr++
				}
				collected++
				wg.Done()
			}()
		}

	case modeDirect:
		addresses, err := ksm.getEndpointIP(metricPortName, telemetryPortName)
		if err != nil {
			ksm.check.AddText("collect_ksm_state", cgm.Tags{
				cgm.Tag{Category: "cluster", Value: ksm.config.Name},
				cgm.Tag{Category: "source", Value: release.NAME},
			}, err.Error())
			ksm.log.Error().Err(err).Msg("getting ksm addresses")
			return
		}
		if metricPortName != "" {
			if addr, ok := addresses[metricPortName]; ok && addr != "" {
				wg.Add(1)
				go func() {
					proto := "http"
					if strings.HasPrefix(metricPortName, "https-") {
						proto = "https"
					}
					metricURL := fmt.Sprintf("%s://%s/metrics", proto, addr)
					if err := ksm.metrics(ctx, metricURL); err != nil {
						ksm.log.Error().Err(err).Str("url", metricURL).Msg("http-metrics")
						collectErr++
					}
					collected++
					wg.Done()
				}()
			}
		}
		if telemetryPortName != "" {
			if addr, ok := addresses[telemetryPortName]; ok && addr != "" {
				wg.Add(1)
				go func() {
					proto := "http"
					if strings.HasPrefix(telemetryPortName, "https-") {
						proto = "https"
					}
					telemetryURL := fmt.Sprintf("%s://%s/metrics", proto, addr)
					if err := ksm.telemetry(ctx, telemetryURL); err != nil {
						ksm.log.Error().Err(err).Str("url", telemetryURL).Msg("telemetry")
						collectErr++
					}
					collected++
					wg.Done()
				}()
			}
		}
	default:
		ksm.check.AddText("collect_ksm_state", cgm.Tags{
			cgm.Tag{Category: "cluster", Value: ksm.config.Name},
			cgm.Tag{Category: "source", Value: release.NAME},
		}, "invalid request mode "+ksm.config.KSMRequestMode)
		ksm.log.Warn().Str("mode", ksm.config.KSMRequestMode).Msg("unknown request mode, skipping ksm collection")
		return
	}

	wg.Wait()

	ksm.check.AddText("collect_ksm_state", cgm.Tags{
		cgm.Tag{Category: "cluster", Value: ksm.config.Name},
		cgm.Tag{Category: "source", Value: release.NAME},
	}, fmt.Sprintf("OK:%d,ERR:%d", collected, collectErr))

	ksm.check.AddHistSample("collect_latency", cgm.Tags{
		cgm.Tag{Category: "source", Value: release.NAME},
		cgm.Tag{Category: "op", Value: "collect_kube-state-metrics"},
		cgm.Tag{Category: "units", Value: "milliseconds"},
	}, float64(time.Since(collectStart).Milliseconds()))

	ksm.log.Debug().Str("duration", time.Since(collectStart).String()).Msg("kube-state-metrics collect end")
	ksm.Lock()
	ksm.running = false
	ksm.Unlock()
}

func (ksm *KSM) getEndpointIP(metricPortName, telemetryPortName string) (map[string]string, error) {
	if ksm.config.KSMFieldSelectorQuery == "" {
		ksm.log.Error().
			Str("field_selector_query", ksm.config.KSMFieldSelectorQuery).
			Msg("invalid service definition, KSM field selectory query not found")
		return nil, errors.New("invalid service definition, missing KSM field selector query")
	}

	clientset, err := k8s.GetClient(ksm.config)
	if err != nil {
		return nil, err
	}

	endpoints, err := clientset.CoreV1().Endpoints("").List(metav1.ListOptions{FieldSelector: ksm.config.KSMFieldSelectorQuery})
	if err != nil {
		return nil, err
	}

	urls := make(map[string]string)
	metricAddress := ""
	telemetryAddress := ""

	for _, endpoint := range endpoints.Items {
		for _, subset := range endpoint.Subsets {
			if len(subset.Addresses) == len(subset.Ports) {
				// it's 1:1 addr[0] goes with port[0], addr[1] with port[1], etc.
				for idx, addr := range subset.Addresses {
					switch subset.Ports[idx].Name {
					case metricPortName:
						metricAddress = fmt.Sprintf("%s:%d", addr.IP, subset.Ports[idx].Port)
					case telemetryPortName:
						telemetryAddress = fmt.Sprintf("%s:%d", addr.IP, subset.Ports[idx].Port)
					}
				}
			} else if len(subset.Addresses) == 1 && len(subset.Ports) > 1 {
				// all ports go with the one address
				for _, port := range subset.Ports {
					switch port.Name {
					case metricPortName:
						metricAddress = fmt.Sprintf("%s:%d", subset.Addresses[0].IP, port.Port)
					case telemetryPortName:
						telemetryAddress = fmt.Sprintf("%s:%d", subset.Addresses[0].IP, port.Port)

					}
				}
			}
		}
	}

	urls[metricPortName] = metricAddress
	urls[telemetryPortName] = telemetryAddress

	return urls, nil
}

func (ksm *KSM) getServiceDefinition() (*v1.Service, error) {
	if ksm.config.KSMFieldSelectorQuery == "" {
		ksm.log.Error().
			Str("field_selector_query", ksm.config.KSMFieldSelectorQuery).
			Msg("invalid service definition, KSM field selectory query not found")
		return nil, errors.New("invalid service definition, missing KSM field selector query")
	}

	clientset, err := k8s.GetClient(ksm.config)
	if err != nil {
		return nil, err
	}

	services, err := clientset.CoreV1().Services("").List(metav1.ListOptions{FieldSelector: ksm.config.KSMFieldSelectorQuery})
	if err != nil {
		ksm.check.IncrementCounter("collect_api_errors", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "request", Value: "kube-state-metrics_service"},
			cgm.Tag{Category: "target", Value: "api-server"},
		})
		return nil, err
	}

	if len(services.Items) == 0 {
		return nil, errors.New("no 'kube-state-metrics' service found")
	}

	if len(services.Items) > 1 {
		return nil, fmt.Errorf("multiple (%d) 'kube-state-metrics' services found", len(services.Items))
	}

	svc := services.Items[0]

	return &svc, nil
}

func (ksm *KSM) metrics(ctx context.Context, metricURL string) error {
	ksm.log.Debug().Str("mode", ksm.config.KSMRequestMode).Str("url", metricURL).Msg("metrics")

	var data *bytes.Reader

	start := time.Now()

	switch ksm.config.KSMRequestMode {
	case modeProxy:
		clientset, err := k8s.GetClient(ksm.config)
		if err != nil {
			return err
		}

		req := clientset.CoreV1().RESTClient().Get().RequestURI(metricURL)
		res := req.Do()
		d, err := res.Raw()
		if err != nil {
			ksm.check.IncrementCounter("collect_api_errors", cgm.Tags{
				cgm.Tag{Category: "source", Value: release.NAME},
				cgm.Tag{Category: "request", Value: "metrics"},
				cgm.Tag{Category: "mode", Value: ksm.config.KSMRequestMode},
				cgm.Tag{Category: "target", Value: "kube-state-metrics"},
			})
			ksm.log.Error().Err(err).Str("url", req.URL().String()).Msg("metrics")
			return err
		}

		data = bytes.NewReader(d)

	case modeDirect:
		client := &http.Client{}
		if strings.HasPrefix(metricURL, "https:") {
			client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec
		}
		req, err := http.NewRequestWithContext(ctx, "GET", metricURL, nil)
		if err != nil {
			return errors.Wrap(err, "/metrics req")
		}
		req.Header.Add("User-Agent", release.NAME+"/"+release.VERSION)
		defer client.CloseIdleConnections()
		resp, err := client.Do(req)
		if err != nil {
			ksm.check.IncrementCounter("collect_api_errors", cgm.Tags{
				cgm.Tag{Category: "source", Value: release.NAME},
				cgm.Tag{Category: "request", Value: "metrics"},
				cgm.Tag{Category: "mode", Value: ksm.config.KSMRequestMode},
				cgm.Tag{Category: "target", Value: "kube-state-metrics"},
			})
			return err
		}
		defer resp.Body.Close()
		ksm.check.AddHistSample("collect_latency", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "request", Value: "metrics"},
			cgm.Tag{Category: "mode", Value: ksm.config.KSMRequestMode},
			cgm.Tag{Category: "target", Value: "kube-state-metrics"},
			cgm.Tag{Category: "units", Value: "milliseconds"},
		}, float64(time.Since(start).Milliseconds()))

		d, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			ksm.log.Error().Err(err).Str("url", metricURL).Msg("reading response")
			return err
		}
		if resp.StatusCode != http.StatusOK {
			ksm.check.IncrementCounter("collect_api_errors", cgm.Tags{
				cgm.Tag{Category: "source", Value: release.NAME},
				cgm.Tag{Category: "request", Value: "metrics"},
				cgm.Tag{Category: "mode", Value: ksm.config.KSMRequestMode},
				cgm.Tag{Category: "target", Value: "kube-state-metrics"},
				cgm.Tag{Category: "code", Value: fmt.Sprintf("%d", resp.StatusCode)},
			})
			ksm.log.Warn().Str("status", resp.Status).RawJSON("response", d).Msg("error from API server")
			return errors.New("error response from api server")
		}
		data = bytes.NewReader(d)
	}

	streamTags := []string{
		"source:kube-state-metrics",
		"source_type:metrics",
		"__rollup:false", // prevent high cardinality metrics from rolling up
	}
	measurementTags := []string{}

	var parser expfmt.TextParser
	if err := ksm.queueMetrics(ctx, parser, ksm.check, ksm.log, data, streamTags, measurementTags, ksm.ts); err != nil {
		return err
	}

	return nil
}

func (ksm *KSM) telemetry(ctx context.Context, telemetryURL string) error {
	ksm.log.Debug().Str("mode", ksm.config.KSMRequestMode).Str("url", telemetryURL).Msg("telemetry")

	var data *bytes.Reader

	start := time.Now()

	switch ksm.config.KSMRequestMode {
	case modeProxy:
		clientset, err := k8s.GetClient(ksm.config)
		if err != nil {
			return err
		}

		req := clientset.CoreV1().RESTClient().Get().RequestURI(telemetryURL)
		res := req.Do()
		d, err := res.Raw()
		if err != nil {
			ksm.check.IncrementCounter("collect_api_errors", cgm.Tags{
				cgm.Tag{Category: "source", Value: release.NAME},
				cgm.Tag{Category: "request", Value: "telemetry"},
				cgm.Tag{Category: "mode", Value: ksm.config.KSMRequestMode},
				cgm.Tag{Category: "target", Value: "kube-state-metrics"},
			})
			ksm.log.Error().Err(err).Str("url", req.URL().String()).Msg("telemetry")
			return err
		}

		data = bytes.NewReader(d)

	case modeDirect:
		client := &http.Client{}
		if strings.HasPrefix(telemetryURL, "https:") {
			client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec
		}
		req, err := http.NewRequestWithContext(ctx, "GET", telemetryURL, nil)
		if err != nil {
			return errors.Wrap(err, "/telemetry req")
		}
		req.Header.Add("User-Agent", release.NAME+"/"+release.VERSION)
		defer client.CloseIdleConnections()

		resp, err := client.Do(req)
		if err != nil {
			ksm.check.IncrementCounter("collect_api_errors", cgm.Tags{
				cgm.Tag{Category: "source", Value: release.NAME},
				cgm.Tag{Category: "request", Value: "telemetry"},
				cgm.Tag{Category: "mode", Value: ksm.config.KSMRequestMode},
				cgm.Tag{Category: "target", Value: "kube-state-metrics"},
			})
			return err
		}
		defer resp.Body.Close()

		ksm.check.AddHistSample("collect_latency", cgm.Tags{
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "request", Value: "telemetry"},
			cgm.Tag{Category: "mode", Value: ksm.config.KSMRequestMode},
			cgm.Tag{Category: "target", Value: "kube-state-metrics"},
			cgm.Tag{Category: "units", Value: "milliseconds"},
		}, float64(time.Since(start).Milliseconds()))

		d, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			ksm.log.Error().Err(err).Str("url", telemetryURL).Msg("reading response")
			return err
		}

		if resp.StatusCode != http.StatusOK {
			ksm.check.IncrementCounter("collect_api_errors", cgm.Tags{
				cgm.Tag{Category: "source", Value: release.NAME},
				cgm.Tag{Category: "request", Value: "telemetry"},
				cgm.Tag{Category: "mode", Value: ksm.config.KSMRequestMode},
				cgm.Tag{Category: "target", Value: "kube-state-metrics"},
				cgm.Tag{Category: "code", Value: fmt.Sprintf("%d", resp.StatusCode)},
			})
			ksm.log.Warn().Str("status", resp.Status).RawJSON("response", d).Msg("error from API server")
			return errors.New("error response from api server")
		}

		data = bytes.NewReader(d)
	}

	streamTags := []string{
		"source:kube-state-metrics",
		"source_type:telemetry",
		"__rollup:false", // prevent high cardinality metrics from rolling up
	}
	measurementTags := []string{}

	var parser expfmt.TextParser
	if err := ksm.queueMetrics(ctx, parser, ksm.check, ksm.log, data, streamTags, measurementTags, ksm.ts); err != nil {
		return err
	}

	return nil
}
