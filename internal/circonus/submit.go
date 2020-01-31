// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package circonus

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"path"
	"strconv"
	"time"

	"code.cloudfoundry.org/bytefmt"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/release"
	"github.com/google/uuid"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type TrapResult struct {
	CheckUUID  string
	SubmitUUID uuid.UUID
	Stats      uint64 `json:"stats"`
	Error      string `json:"error,omitempty"`
}

const (
	compressionThreshold = 1024
	traceTSFormat        = "20060102_150405.000000000"
)

func (c *Check) ResetSubmitStats() {
	c.statsmu.Lock()
	defer c.statsmu.Unlock()
	c.stats.Metrics = 0
	c.stats.SentBytes = 0
}

func (c *Check) SubmitStats() Stats {
	c.statsmu.Lock()
	defer c.statsmu.Unlock()
	return Stats{
		Metrics:   c.stats.Metrics,
		SentBytes: c.stats.SentBytes,
		SentSize:  bytefmt.ByteSize(c.stats.SentBytes),
	}
}

func (c *Check) SubmitQueue(ctx context.Context, metrics map[string]MetricSample, resultLogger zerolog.Logger) error {
	if metrics == nil {
		return errors.New("invalid metrics (nil)")
	}

	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return errors.Wrap(err, "marshaling metrics")
	}

	return c.SubmitStream(ctx, bytes.NewReader(data), resultLogger)
}

// SubmitStream sends metrics to a circonus trap
func (c *Check) SubmitStream(ctx context.Context, metrics io.Reader, resultLogger zerolog.Logger) error {
	if metrics == nil {
		return errors.New("invalid metrics (nil)")
	}

	start := time.Now()

	if c.submissionURL == "" {
		if c.config.DryRun {
			_, err := io.Copy(os.Stdout, metrics)
			return err
		}
		return errors.New("no submission url and not in dry-run mode")
	}

	var client *http.Client

	if c.brokerTLSConfig != nil {
		client = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 30 * time.Second,
					DualStack: true,
				}).DialContext,
				TLSClientConfig:     c.brokerTLSConfig,
				TLSHandshakeTimeout: 10 * time.Second,
				DisableKeepAlives:   false,
				MaxIdleConnsPerHost: 2,
				DisableCompression:  false,
			},
		}
	} else {
		client = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 30 * time.Second,
					DualStack: true,
				}).DialContext,
				DisableKeepAlives:   false,
				MaxIdleConnsPerHost: 2,
				DisableCompression:  false,
			},
		}
	}

	submitUUID, err := uuid.NewRandom()
	if err != nil {
		return errors.Wrap(err, "creating new submit ID")
	}

	rawData, err := ioutil.ReadAll(metrics)
	if err != nil {
		return errors.Wrap(err, "reading metric data")
	}

	payloadIsCompressed := false

	var subData *bytes.Buffer
	if c.UseCompression() && len(rawData) > compressionThreshold {
		subData = bytes.NewBuffer([]byte{})
		zw := gzip.NewWriter(subData)
		n, err := zw.Write(rawData)
		if err != nil {
			return errors.Wrap(err, "compressing metrics")
		}
		if n != len(rawData) {
			return errors.Errorf("write length mismatch data length %d != written length %d", len(rawData), n)
		}
		if err := zw.Close(); err != nil {
			return errors.Wrap(err, "closing gzip writer")
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
		fh, err := os.Create(fn)
		if err != nil {
			c.log.Error().Err(err).Str("file", fn).Msg("skipping submit trace")
		} else {
			if _, err := fh.Write(subData.Bytes()); err != nil {
				c.log.Error().Err(err).Msg("writing metric trace")
			}
			if err := fh.Close(); err != nil {
				c.log.Error().Err(err).Str("file", fn).Msg("closing metric trace")
			}
		}
	}

	dataLen := subData.Len()

	req, err := retryablehttp.NewRequest("PUT", c.submissionURL, subData)
	if err != nil {
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

	if c.DebugSubmissions() {
		dump, err := httputil.DumpRequestOut(req.Request, !payloadIsCompressed)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(string(dump))
	}

	retryClient := retryablehttp.NewClient()
	retryClient.HTTPClient = client
	retryClient.Logger = logshim{logh: c.log.With().Str("pkg", "retryablehttp").Logger()}
	retryClient.RetryWaitMin = 50 * time.Millisecond
	retryClient.RetryWaitMax = 2 * time.Second
	retryClient.RetryMax = 10
	retryClient.RequestLogHook = func(l retryablehttp.Logger, r *http.Request, attempt int) {
		if attempt > 0 {
			l.Printf("[WARN] %s retry %d", r.URL, attempt)
		}
	}

	defer retryClient.HTTPClient.CloseIdleConnections()

	resp, err := retryClient.Do(req)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		c.log.Error().Str("url", c.submissionURL).Str("status", resp.Status).Str("body", string(body)).Msg("submitting telemetry")
		return errors.Errorf("submitting metrics (%s %s)", c.submissionURL, resp.Status)
	}

	var result TrapResult
	if err := json.Unmarshal(body, &result); err != nil {
		return errors.Wrapf(err, "parsing response (%s)", string(body))
	}

	result.CheckUUID = c.checkUUID
	result.SubmitUUID = submitUUID

	resultLogger.Debug().
		Str("duration", time.Since(start).String()).
		Interface("result", result).
		Str("bytes_sent", bytefmt.ByteSize(uint64(dataLen))).
		Msg("submitted")

	c.statsmu.Lock()
	c.stats.Metrics += result.Stats
	c.stats.SentBytes += uint64(dataLen)
	c.statsmu.Unlock()

	return nil
}
