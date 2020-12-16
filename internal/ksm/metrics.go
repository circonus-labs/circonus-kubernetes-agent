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

	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/circonus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

const (
	// NOTE: these will be enabled in a future release when we support cumulative histograms (type H)
	//       flip emitHistogramBuckets to true and leave circCumulativeHistogram true. Once we're
	//       happy with the support the gating flag logic can be removed and the code simplifed.
	emitHistogramBuckets    = true
	circCumulativeHistogram = true
)

// queueMetrics is a generic function to digest prometheus text format metrics and
// emit circonus formatted metrics.
// Formats supported: https://prometheus.io/docs/instrumenting/exposition_formats/
func (ksm *KSM) queueMetrics(
	ctx context.Context,
	ksmSource string,
	parser expfmt.TextParser,
	check *circonus.Check,
	data io.Reader,
	parentStreamTags []string,
	parentMeasurementTags []string) error {

	srcLogger := ksm.log.With().Str("ksm_source", ksmSource).Logger()

	var baseStreamTags []string
	if len(parentStreamTags) > 0 {
		baseStreamTags = make([]string, len(parentStreamTags))
		copy(baseStreamTags, parentStreamTags)
	}

	metricFamilies, err := parser.TextToMetricFamilies(data)
	if err != nil {
		return err
	}

	metricsProcessed := 0
	metrics := make(map[string]circonus.MetricSample)
	for mn, mf := range metricFamilies {
		if done(ctx) {
			return nil
		}
		for _, m := range mf.Metric {
			metricsProcessed++
			if done(ctx) {
				return nil
			}
			metricName := mn
			streamTags := check.NewTagList(baseStreamTags, getLabels(m))
			switch mf.GetType() {
			case dto.MetricType_SUMMARY:
				_ = check.QueueMetricSample(
					metrics, metricName+"_count",
					circonus.MetricTypeUint64,
					streamTags, parentMeasurementTags,
					m.GetSummary().GetSampleCount(), ksm.ts)
				_ = check.QueueMetricSample(
					metrics, metricName+"_sum",
					circonus.MetricTypeFloat64,
					streamTags, parentMeasurementTags,
					m.GetSummary().GetSampleSum(), ksm.ts)
				for qn, qv := range getQuantiles(m) {
					qtags := check.NewTagList(streamTags, []string{"quantile:" + qn})
					_ = check.QueueMetricSample(
						metrics, metricName,
						circonus.MetricTypeFloat64,
						qtags, parentMeasurementTags,
						qv, ksm.ts)
				}
			case dto.MetricType_HISTOGRAM:
				_ = check.QueueMetricSample(
					metrics, metricName+"_count",
					circonus.MetricTypeUint64,
					streamTags, parentMeasurementTags,
					m.GetHistogram().GetSampleCount(), ksm.ts)
				_ = check.QueueMetricSample(
					metrics, metricName+"_sum",
					circonus.MetricTypeFloat64,
					streamTags, parentMeasurementTags,
					m.GetHistogram().GetSampleSum(), ksm.ts)
				// add average, requested for dns dashboard
				_ = check.QueueMetricSample(
					metrics, metricName+"_avg",
					circonus.MetricTypeFloat64,
					streamTags, parentMeasurementTags,
					m.GetHistogram().GetSampleSum()/float64(m.GetHistogram().GetSampleCount()), ksm.ts)

				if emitHistogramBuckets {
					if circCumulativeHistogram {
						histo := promHistoBucketsToCircHisto(m)
						if len(histo) > 0 {
							_ = check.QueueMetricSample(
								metrics, metricName,
								circonus.MetricTypeCumulativeHistogram,
								streamTags, parentMeasurementTags,
								histo, ksm.ts)
						}
					} else {
						for bn, bv := range getBuckets(m) {
							htags := check.NewTagList(streamTags, []string{"bucket:" + bn})
							_ = check.QueueMetricSample(
								metrics, metricName,
								circonus.MetricTypeUint64,
								htags, parentMeasurementTags,
								bv, ksm.ts)
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
						// text metrics removed to reduce load - not used in dashboard
						// reason, fullTags, countTags := keyTags(check, streamTags, check.DefaultCGMTags(), "reason")

						_, _, countTags := keyTags(check, streamTags, check.DefaultCGMTags(), "reason")
						ksm.cgmMetrics.IncrementByValueWithTags(customMetricName+"_count", countTags, uint64(val))
						// pod_container_status_waiting_reason_count
						// pod_init_container_status_waiting_reason_count
						// pod_container_status_terminated_reason_count
						// pod_init_container_status_terminated_reason_count

						// if val > 0 {
						// 	ksm.cgmMetrics.SetTextValueWithTags(customMetricName, fullTags, reason)
						// }
						// continue -- when original ksm metric no longer needed
					case "kube_pod_status_phase":
						phase, fullTags, countTags := keyTags(check, streamTags, check.DefaultCGMTags(), "phase")
						ksm.cgmMetrics.IncrementByValueWithTags(customMetricName+"_count", countTags, uint64(val))
						// pod_status_phase_count
						if val > 0 {
							ksm.cgmMetrics.SetTextValueWithTags(customMetricName, fullTags, phase)
							// pod_status_phase
						}
						// continue -- when original ksm metric no longer needed
					case "kube_pod_status_ready", "kube_pod_status_scheduled":
						// text metrics removed to reduce load - not used in dashboard
						// condition, fullTags, countTags := keyTags(check, streamTags, check.DefaultCGMTags(), "condition")
						_, _, countTags := keyTags(check, streamTags, check.DefaultCGMTags(), "condition")
						ksm.cgmMetrics.IncrementByValueWithTags(customMetricName+"_count", countTags, uint64(val))
						// pod_status_ready_count
						// pod_status_scheduled_count

						// if val > 0 {
						// 	ksm.cgmMetrics.SetTextValueWithTags(customMetricName, fullTags, condition)
						// }
						// continue -- when original ksm metric no longer needed
					case "kube_pod_container_status_running",
						"kube_pod_container_status_terminated",
						"kube_pod_container_status_waiting",
						"kube_pod_container_status_ready":
						// text metrics removed to reduce load - not used in dashboard
						// _, fullTags, countTags := keyTags(check, streamTags, check.DefaultCGMTags(), "")
						_, _, countTags := keyTags(check, streamTags, check.DefaultCGMTags(), "")
						ksm.cgmMetrics.IncrementByValueWithTags(customMetricName+"_count", countTags, uint64(val))
						// pod_container_status_running_count
						// pod_container_status_terminated_count
						// pod_container_status_waiting_count
						// pod_container_status_ready_count

						// if val > 0 {
						// 	shortName := "pod_container_status"
						// 	switch metricName {
						// 	case "kube_pod_container_status_terminated":
						// 		ksm.cgmMetrics.SetTextValueWithTags(shortName, fullTags, "terminated")
						// 	case "kube_pod_container_status_waiting":
						// 		ksm.cgmMetrics.SetTextValueWithTags(shortName, fullTags, "waiting")
						// 	case "kube_pod_container_status_running":
						// 		ksm.cgmMetrics.SetTextValueWithTags(shortName, fullTags, "running")
						// 	case "kube_pod_container_status_ready":
						// 		ksm.cgmMetrics.SetTextValueWithTags(shortName, fullTags, "ready")
						// 	}
						// }
						// continue -- when original ksm metric no longer needed
					}
					_ = check.QueueMetricSample(
						metrics, metricName,
						circonus.MetricTypeFloat64,
						streamTags, parentMeasurementTags,
						val, ksm.ts)
				}
			case dto.MetricType_COUNTER:
				if m.GetCounter().Value != nil {
					_ = check.QueueMetricSample(
						metrics, metricName,
						circonus.MetricTypeFloat64,
						streamTags, parentMeasurementTags,
						m.GetCounter().GetValue(), ksm.ts)
				}
			case dto.MetricType_UNTYPED:
				if m.GetUntyped().Value != nil {
					if *m.GetUntyped().Value == math.Inf(+1) {
						srcLogger.Warn().
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
						m.GetUntyped().GetValue(), ksm.ts)
				}
			}
		}
	}

	// add derived metrics
	m := ksm.cgmMetrics.FlushMetrics()
	for mn, mv := range *m {
		switch mv.Type {
		case circonus.MetricTypeString:
			_ = check.QueueMetricSample(
				metrics, mn,
				circonus.MetricTypeString,
				[]string{}, []string{},
				mv.Value, ksm.ts)
		case circonus.MetricTypeUint64:
			_ = check.QueueMetricSample(
				metrics, mn,
				circonus.MetricTypeUint64,
				[]string{}, []string{},
				mv.Value, ksm.ts)
		default:
			srcLogger.Warn().Str("name", mn).Interface("mv", mv).Msg("unrecognized metric type")
		}
	}

	if len(metrics) == 0 {
		srcLogger.Warn().Int("metrics_processed", metricsProcessed).Msg("zero metrics to submit")
		if metricsProcessed == 0 {
			// if there are none to send and none were processed, this indicates KSM may be having issues
			return fmt.Errorf("zero metrics received from KSM - check KSM logs for errors")
		}
		return nil
	}

	if err := check.SubmitMetrics(ctx, metrics, srcLogger, true); err != nil {
		srcLogger.Warn().Err(err).Msg("submitting metrics")
	}

	return nil
}

func getLabels(m *dto.Metric) []string {
	labels := make([]string, len(m.Label))
	idx := 0

	for _, label := range m.Label {
		if label.Name == nil || *label.Name == "" {
			continue
		}
		if label.Value == nil || *label.Value == "" {
			continue
		}

		labels[idx] = *label.Name + ":" + *label.Value
		idx++
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

func keyTags(check *circonus.Check, originalTags []string, baseTags cgm.Tags, keyCat string) (string, cgm.Tags, cgm.Tags) {
	keyVal := ""
	tags := check.TagListToCGM(originalTags)

	fullTags := make(cgm.Tags, len(baseTags)+len(tags))
	copy(fullTags, baseTags)

	countTags := make(cgm.Tags, len(baseTags)+len(tags))
	copy(countTags, baseTags)

	fidx := len(baseTags)
	cidx := len(baseTags)
	for _, tag := range tags {
		if keyCat != "" && tag.Category == keyCat {
			countTags[cidx] = tag
			cidx++
			keyVal = tag.Value
			continue
		}
		if tag.Category == "pod" || tag.Category == "container" {
			fullTags[fidx] = tag
			fidx++
			continue
		}
		fullTags[fidx] = tag
		fidx++
		countTags[cidx] = tag
		cidx++
	}
	return keyVal, fullTags, countTags
}

// makeTimestamp returns timestamp in ms units for _ts metric value
// func makeTimestamp(ts *time.Time) uint64 {
// 	return uint64(ts.UTC().UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond)))
// }
