// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package ksm

import (
	"context"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/circonus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/rs/zerolog"
)

const (
	// NOTE: these will be enabled in a future release when we support cumulative histograms (type H)
	//       flip emitHistogramBuckets to true and leave circCumulativeHistogram true. Once we're
	//       happy with the support the gating flag logic can be removed and the code simplifed.
	emitHistogramBuckets    = false
	circCumulativeHistogram = true
)

// queueMetrics is a generic function to digest prometheus text format metrics and
// emit circonus formatted metrics.
// Formats supported: https://prometheus.io/docs/instrumenting/exposition_formats/
func queueMetrics(
	ctx context.Context,
	parser expfmt.TextParser,
	check *circonus.Check,
	logger zerolog.Logger,
	data io.Reader,
	parentStreamTags []string,
	parentMeasurementTags []string,
	ts *time.Time) error {

	var baseStreamTags []string
	if len(parentStreamTags) > 0 {
		baseStreamTags = make([]string, len(parentStreamTags))
		copy(baseStreamTags, parentStreamTags)
	}

	metricFamilies, err := parser.TextToMetricFamilies(data)
	if err != nil {
		return err
	}

	metrics := make(map[string]circonus.MetricSample)

	for mn, mf := range metricFamilies {
		if done(ctx) {
			return nil
		}
		for _, m := range mf.Metric {
			if done(ctx) {
				return nil
			}
			metricName := mn
			streamTags := getLabels(m)
			streamTags = append(streamTags, baseStreamTags...)
			switch mf.GetType() {
			case dto.MetricType_SUMMARY:
				_ = check.QueueMetricSample(
					metrics, metricName+"_count",
					circonus.MetricTypeUint64,
					streamTags, parentMeasurementTags,
					m.GetSummary().GetSampleCount(), ts)
				_ = check.QueueMetricSample(
					metrics, metricName+"_sum",
					circonus.MetricTypeFloat64,
					streamTags, parentMeasurementTags,
					m.GetSummary().GetSampleSum(), ts)
				for qn, qv := range getQuantiles(m) {
					var qtags []string
					qtags = append(qtags, streamTags...)
					qtags = append(qtags, "quantile:"+qn)
					_ = check.QueueMetricSample(
						metrics, metricName,
						circonus.MetricTypeFloat64,
						qtags, parentMeasurementTags,
						qv, ts)
				}
			case dto.MetricType_HISTOGRAM:
				_ = check.QueueMetricSample(
					metrics, metricName+"_count",
					circonus.MetricTypeUint64,
					streamTags, parentMeasurementTags,
					m.GetHistogram().GetSampleCount(), ts)
				_ = check.QueueMetricSample(
					metrics, metricName+"_sum",
					circonus.MetricTypeFloat64,
					streamTags, parentMeasurementTags,
					m.GetHistogram().GetSampleSum(), ts)
				// add average, requested for dns dashboard
				_ = check.QueueMetricSample(
					metrics, metricName+"_avg",
					circonus.MetricTypeFloat64,
					streamTags, parentMeasurementTags,
					m.GetHistogram().GetSampleSum()/float64(m.GetHistogram().GetSampleCount()), ts)

				if emitHistogramBuckets {
					if circCumulativeHistogram {
						var htags []string
						htags = append(htags, streamTags...)
						histo := promHistoBucketsToCircHisto(m)
						if len(histo) > 0 {
							_ = check.QueueMetricSample(
								metrics, metricName,
								circonus.MetricTypeCumulativeHistogram,
								htags, parentMeasurementTags,
								strings.Join(histo, ","), ts)
						}
					} else {
						for bn, bv := range getBuckets(m) {
							var htags []string
							htags = append(htags, streamTags...)
							htags = append(htags, "bucket:"+bn)
							_ = check.QueueMetricSample(
								metrics, metricName,
								circonus.MetricTypeUint64,
								htags, parentMeasurementTags,
								bv, ts)
						}
					}
				}
			case dto.MetricType_GAUGE:
				if m.GetGauge().Value != nil {
					val := m.GetGauge().GetValue()
					customMetricName := strings.Replace(metricName, "kube_", "", 1)
					switch metricName {
					case "kube_pod_container_status_waiting_reason", "kube_pod_init_container_status_waiting_reason":
						fallthrough
					case "kube_pod_container_status_terminated_reason", "kube_pod_init_container_status_terminated_reason":
						reason, fullTags, countTags := keyTags(check, streamTags, "reason")
						check.IncrementCounterByValue(customMetricName+"_count", countTags, uint64(val))
						if val > 0 {
							check.AddText(customMetricName, fullTags, reason)
						}
						// continue -- when original ksm metric no longer needed
					case "kube_pod_status_phase":
						phase, fullTags, countTags := keyTags(check, streamTags, "phase")
						// logger.Warn().Str("metric", customMetricName).Interface("tags", countTags).Float64("val", val).Msg("pod status phase")
						check.IncrementCounterByValue(customMetricName+"_count", countTags, uint64(val))
						if val > 0 {
							check.AddText(customMetricName, fullTags, phase)
						}
						// continue -- when original ksm metric no longer needed
					case "kube_pod_status_ready", "kube_pod_status_scheduled":
						_, fullTags, countTags := keyTags(check, streamTags, "condition")
						check.IncrementCounterByValue(customMetricName+"_count", countTags, uint64(val))
						if val > 0 {
							shortName := "pod_status"
							switch metricName {
							case "kube_pod_status_ready":
								check.AddText(shortName, fullTags, "ready")
							case "kube_pod_status_scheduled":
								check.AddText(shortName, fullTags, "scheduled")
							}
						}
						// continue -- when original ksm metric no longer needed
					case "kube_pod_container_status_running", "kube_pod_container_status_terminated", "kube_pod_container_status_waiting", "kube_pod_container_status_ready":
						_, fullTags, countTags := keyTags(check, streamTags, "")
						check.IncrementCounterByValue(customMetricName+"_count", countTags, uint64(val))
						if val > 0 {
							shortName := "pod_container_status"
							switch metricName {
							case "kube_pod_container_status_running":
								check.AddText(shortName, fullTags, "running")
							case "kube_pod_container_status_terminated":
								check.AddText(shortName, fullTags, "terminated")
							case "kube_pod_container_status_waiting":
								check.AddText(shortName, fullTags, "waiting")
							case "kube_pod_container_status_ready":
								check.AddText(shortName, fullTags, "ready")
							}
						}
						// continue -- when original ksm metric no longer needed
					}
					_ = check.QueueMetricSample(
						metrics, metricName,
						circonus.MetricTypeFloat64,
						streamTags, parentMeasurementTags,
						val, ts)
				}
			case dto.MetricType_COUNTER:
				if m.GetCounter().Value != nil {
					_ = check.QueueMetricSample(
						metrics, metricName,
						circonus.MetricTypeFloat64,
						streamTags, parentMeasurementTags,
						m.GetCounter().GetValue(), ts)
				}
			case dto.MetricType_UNTYPED:
				if m.GetUntyped().Value != nil {
					if *m.GetUntyped().Value == math.Inf(+1) {
						logger.Warn().
							Str("metric", metricName).
							Str("type", mf.GetType().String()).
							Str("value", (*m).GetUntyped().String()).
							Msg("cannot coerce +Inf to uint64")
						continue
					}
					_ = check.QueueMetricSample(
						metrics, metricName,
						circonus.MetricTypeFloat64,
						streamTags, parentMeasurementTags,
						m.GetUntyped().GetValue(), ts)
				}
			}
		}
	}

	if len(metrics) == 0 {
		return nil
	}

	if err := check.SubmitMetrics(ctx, metrics, logger, true); err != nil {
		logger.Warn().Err(err).Msg("submitting metrics")
	}

	return nil
}

func getLabels(m *dto.Metric) []string {
	labels := []string{}

	for _, label := range m.Label {
		if label.Name == nil || *label.Name == "" {
			continue
		}
		if label.Value == nil || *label.Value == "" {
			continue
		}

		labels = append(labels, *label.Name+":"+*label.Value)
	}

	return labels
}

func getQuantiles(m *dto.Metric) map[string]float64 {
	ret := make(map[string]float64)
	for _, q := range m.GetSummary().Quantile {
		if q.Value != nil && !math.IsNaN(*q.Value) {
			ret[fmt.Sprint(*q.Quantile)] = *q.Value
		}
	}
	return ret
}

func getBuckets(m *dto.Metric) map[string]uint64 {
	ret := make(map[string]uint64)
	for _, b := range m.GetHistogram().Bucket {
		if b.CumulativeCount != nil {
			ret[fmt.Sprint(*b.UpperBound)] = *b.CumulativeCount
		}
	}
	return ret
}

func promHistoBucketsToCircHisto(m *dto.Metric) []string {
	const reducer = 0.999
	var ret []string
	tot := m.GetHistogram().GetSampleCount()
	n := uint64(0)
	for _, b := range m.GetHistogram().Bucket {
		if b.CumulativeCount != nil {
			v := *b.CumulativeCount - n
			if v > 0 {
				upperBound := *b.UpperBound
				if upperBound == math.Inf(+1) {
					upperBound = 10e+127
				} else {
					upperBound *= reducer
				}
				ret = append(ret, fmt.Sprintf("H[%e]=%d", upperBound, v))
				if *b.CumulativeCount == tot {
					break
				}
				n += v
			}
		}
	}
	return ret
}

func done(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

func keyTags(check *circonus.Check, originalTags []string, keyCat string) (string, cgm.Tags, cgm.Tags) {
	keyVal := ""
	tags := check.TagListToCGM(originalTags)
	fullTags := make(cgm.Tags, 0)
	countTags := make(cgm.Tags, 0)
	for _, tag := range tags {
		if keyCat != "" && tag.Category == keyCat {
			countTags = append(countTags, tag)
			keyVal = tag.Value
			continue
		}
		if tag.Category == "pod" || tag.Category == "container" {
			fullTags = append(fullTags, tag)
			continue
		}
		fullTags = append(fullTags, tag)
		countTags = append(countTags, tag)
	}
	return keyVal, fullTags, countTags
}
