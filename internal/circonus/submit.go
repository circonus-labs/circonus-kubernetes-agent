// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package circonus

import (
	"bytes"
	// "compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/bytefmt"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/release"
	"github.com/google/uuid"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/klauspost/compress/gzip"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type TrapResult struct {
	CheckUUID  string
	Error      string `json:"error,omitempty"`
	Filtered   uint64 `json:"filtered,omitempty"`
	Stats      uint64 `json:"stats"`
	SubmitUUID uuid.UUID
}

const (
	compressionThreshold = 1024
	traceTSFormat        = "20060102_150405.000000000"
)

// FlushCGM sends the tracking metrics collected in a CGM instance within the Check.
func (c *Check) FlushCGM(ctx context.Context, ts *time.Time, lg zerolog.Logger, agentStats bool) {
	if c.metrics != nil {
		// TODO: add timestamp support to CGM (e.g. FlushMetricsWithTimestamp(ts))
		metrics := make(map[string]MetricSample)

		c.metricsmu.Lock()
		for mn, mv := range *(c.metrics.FlushMetrics()) {
			ms := MetricSample{
				Value: mv.Value,
				Type:  mv.Type,
			}
			if ms.Type != MetricTypeHistogram {
				ms.Timestamp = makeTimestamp(ts)
			}
			metrics[mn] = ms
			if strings.HasPrefix(mn, "collect_k8s_event_count") {
				c.metrics.Set(mn, 0) // reset event counter
			}
		}
		c.metricsmu.Unlock()

		if agentStats && c.LogAgentMetrics() {
			data, err := json.Marshal(metrics)
			if err != nil {
				c.log.Warn().Err(err).Msg("marshling agent metrics")
			} else {
				c.log.Info().RawJSON("agent_metrics", data).Msg("before submitting")
			}
		}

		for {
			submitCtx, submitCtxCancel := context.WithDeadline(ctx, time.Now().Add(c.submitDeadline))
			err := c.submitMetrics(submitCtx, metrics, lg, !agentStats)
			if err == nil {
				submitCtxCancel()
				break
			}

			if errors.Is(err, context.DeadlineExceeded) {
				c.log.Warn().Err(err).Str("deadline", c.submitDeadline.String()).Msg("deadline reached submitting metrics, retrying")
				submitCtxCancel()
				continue
			}

			submitCtxCancel()
			c.log.Error().Err(err).Msg("submitting metrics")
			break
		}

	}
}

// FlushCollectorMetrics sends metrics from discrete collectors and sub-collectors
func (c *Check) FlushCollectorMetrics(ctx context.Context, metrics map[string]MetricSample, resultLogger zerolog.Logger, includeStats bool) error {
	var err error

	for {
		submitCtx, submitCtxCancel := context.WithDeadline(ctx, time.Now().Add(c.submitDeadline))
		err = c.submitMetrics(submitCtx, metrics, resultLogger, includeStats)
		if err == nil {
			submitCtxCancel()
			break
		}

		if errors.Is(err, context.DeadlineExceeded) {
			c.log.Warn().Err(err).Str("deadline", c.submitDeadline.String()).Msg("deadline reached submitting metrics, retrying")
			submitCtxCancel()
			continue
		}

		submitCtxCancel()
		c.log.Error().Err(err).Msg("submitting metrics")
		break
	}

	return err
}

// submitMetrics does the heavy lifting to send metric batches to the trap check
func (c *Check) submitMetrics(ctx context.Context, metrics map[string]MetricSample, resultLogger zerolog.Logger, includeStats bool) error {
	if metrics == nil {
		return errors.New("invalid metrics (nil)")
	}
	if len(metrics) == 0 {
		return nil
	}

	baseTags := cgm.Tags{
		cgm.Tag{Category: "source", Value: release.NAME},
	}
	baseTags = append(baseTags, c.defaultTags...)

	rawData, err := json.Marshal(metrics)
	if err != nil {
		resultLogger.Error().Err(err).Msg("json encoding metrics")
		return errors.Wrap(err, "marshaling metrics")
	}

	start := time.Now()

	if c.submissionURL == "" {
		if c.config.DryRun {
			_, err := io.Copy(os.Stdout, bytes.NewReader(rawData))
			return err
		}
		return errors.New("no submission url and not in dry-run mode")
	}

	if c.client == nil {
		if c.brokerTLSConfig != nil {
			c.client = &http.Client{
				Transport: &http.Transport{
					Proxy: http.ProxyFromEnvironment,
					DialContext: (&net.Dialer{
						Timeout:       10 * time.Second,
						KeepAlive:     3 * time.Second,
						FallbackDelay: -1 * time.Millisecond,
					}).DialContext,
					TLSClientConfig:     c.brokerTLSConfig,
					TLSHandshakeTimeout: 10 * time.Second,
					DisableKeepAlives:   true,
					DisableCompression:  false,
					MaxIdleConns:        1,
					MaxIdleConnsPerHost: 0,
				},
				Timeout: 60 * time.Second, // hard 60s timeout
			}
		} else {
			c.client = &http.Client{
				Transport: &http.Transport{
					Proxy: http.ProxyFromEnvironment,
					DialContext: (&net.Dialer{
						Timeout:       10 * time.Second,
						KeepAlive:     3 * time.Second,
						FallbackDelay: -1 * time.Millisecond,
					}).DialContext,
					DisableKeepAlives:   true,
					DisableCompression:  false,
					MaxIdleConns:        1,
					MaxIdleConnsPerHost: 0,
				},
				Timeout: 60 * time.Second, // hard 60s timeout
			}
		}
	}

	submitUUID, err := uuid.NewRandom()
	if err != nil {
		resultLogger.Error().Err(err).Msg("creating new submit ID")
		return errors.Wrap(err, "creating new submit ID")
	}

	payloadIsCompressed := false

	var subData *bytes.Buffer
	if c.UseCompression() && len(rawData) > compressionThreshold {
		subData = bytes.NewBuffer([]byte{})
		zw := gzip.NewWriter(subData)
		n, e1 := zw.Write(rawData)
		if e1 != nil {
			resultLogger.Error().Err(e1).Msg("compressing metrics")
			return errors.Wrap(e1, "compressing metrics")
		}
		if n != len(rawData) {
			resultLogger.Error().Int("data_len", len(rawData)).Int("written", n).Msg("gzip write length mismatch")
			return errors.Errorf("write length mismatch data length %d != written length %d", len(rawData), n)
		}
		if e2 := zw.Close(); e2 != nil {
			resultLogger.Error().Err(e2).Msg("closing gzip writer")
			return errors.Wrap(e2, "closing gzip writer")
		}
		payloadIsCompressed = true
	} else {
		subData = bytes.NewBuffer(rawData)
	}

	if dumpDir := c.config.TraceSubmits; dumpDir != "" {
		fn := path.Join(dumpDir, time.Now().UTC().Format(traceTSFormat)+"_"+submitUUID.String()+".json")
		if payloadIsCompressed {
			fn += ".gz"
		}

		if fh, e1 := os.Create(fn); e1 != nil {
			c.log.Error().Err(e1).Str("file", fn).Msg("skipping submit trace")
		} else {
			if _, e2 := fh.Write(subData.Bytes()); e2 != nil {
				resultLogger.Error().Err(e2).Msg("writing metric trace")
			}
			if e3 := fh.Close(); e3 != nil {
				resultLogger.Error().Err(e3).Str("file", fn).Msg("closing metric trace")
			}
		}
	}

	dataLen := len(rawData) // subData.Len()

	reqStart := time.Now()

	req, err := retryablehttp.NewRequest("PUT", c.submissionURL, subData)
	if err != nil {
		resultLogger.Error().Err(err).Msg("creating submission request")
		return err
	}
	req = req.WithContext(ctx)
	req.Header.Set("User-Agent", release.NAME+"/"+release.VERSION)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Connection", "close")
	req.Header.Set("Content-Length", strconv.Itoa(dataLen))
	if payloadIsCompressed {
		req.Header.Set("Content-Encoding", "gzip")
	}

	retryClient := retryablehttp.NewClient()
	retryClient.HTTPClient = c.client
	retryClient.Logger = submitLogshim{logh: c.log.With().Str("pkg", "retryablehttp").Logger()}
	retryClient.RetryWaitMin = 50 * time.Millisecond
	retryClient.RetryWaitMax = 1 * time.Second
	retryClient.RetryMax = 10
	retryClient.RequestLogHook = func(l retryablehttp.Logger, r *http.Request, attempt int) {
		if attempt > 0 {
			c.metrics.IncrementWithTags("collect_submit_retries", baseTags)
			reqStart = time.Now()
			resultLogger.Warn().Str("url", r.URL.String()).Int("retry", attempt).Msg("retrying...")
		}
	}
	retryClient.ResponseLogHook = func(l retryablehttp.Logger, r *http.Response) {
		c.AddHistSample("collect_latency", cgm.Tags{
			cgm.Tag{Category: "type", Value: "submit"},
			cgm.Tag{Category: "source", Value: release.NAME},
			cgm.Tag{Category: "units", Value: "milliseconds"},
		}, float64(time.Since(reqStart).Milliseconds()))
		if r.StatusCode != http.StatusOK {
			tags := cgm.Tags{cgm.Tag{Category: "code", Value: fmt.Sprintf("%d", r.StatusCode)}}
			tags = append(tags, baseTags...)
			c.metrics.IncrementWithTags("collect_submit_errors", tags)
			resultLogger.Warn().Str("url", r.Request.URL.String()).Str("status", r.Status).Msg("non-200 response...")
		}
	}

	defer retryClient.HTTPClient.CloseIdleConnections()

	resp, err := retryClient.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		resultLogger.Error().Err(err).Msg("making request")
		c.metrics.IncrementWithTags("collect_submit_fails", baseTags)
		return err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		resultLogger.Error().Err(err).Msg("reading body")
		return err
	}

	if resp.StatusCode != http.StatusOK {
		tags := cgm.Tags{cgm.Tag{Category: "code", Value: fmt.Sprintf("%d", resp.StatusCode)}}
		tags = append(tags, baseTags...)
		c.metrics.IncrementWithTags("collect_submit_fails", tags)
		resultLogger.Error().Str("url", c.submissionURL).Str("status", resp.Status).Str("body", string(body)).Msg("submitting telemetry")
		return errors.Errorf("submitting metrics (%s %s)", c.submissionURL, resp.Status)
	}

	c.metrics.IncrementWithTags("collect_submits", baseTags)

	var result TrapResult
	if err := json.Unmarshal(body, &result); err != nil {
		resultLogger.Error().Err(err).Str("body", string(body)).Msg("parsing response")
		return errors.Wrapf(err, "parsing response (%s)", string(body))
	}

	result.CheckUUID = c.checkUUID
	result.SubmitUUID = submitUUID

	if result.Error != "" {
		resultLogger.Warn().Interface("result", result).Msg("error message in result from broker")
	}

	resultLogger.Debug().
		Str("duration", time.Since(start).String()).
		Int("sent_metrics", len(metrics)).
		Interface("result", result).
		Str("bytes_sent", bytefmt.ByteSize(uint64(dataLen))).
		Msg("submitted")

	if includeStats {
		c.statsmu.Lock()
		c.stats.RecvMetrics += result.Stats
		c.stats.SentMetrics += uint64(len(metrics))
		c.stats.SentBytes += uint64(dataLen)
		c.stats.SentSize += uint64(subData.Len())
		c.stats.BkrFiltered += result.Filtered
		c.statsmu.Unlock()
	}

	return nil
}
