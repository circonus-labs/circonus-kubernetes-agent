// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package k8s

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/circonus-labs/circonus-kubernetes-agent/internal/release"
	"github.com/pkg/errors"
)

var client *http.Client

func NewAPIClient(tlscfg *tls.Config, reqTimeout time.Duration) (*http.Client, error) {
	if reqTimeout == time.Duration(0) {
		reqTimeout = 10 * time.Second
	}

	// var client *http.Client

	if client == nil {
		if tlscfg != nil {
			client = &http.Client{
				Timeout: reqTimeout,
				Transport: &http.Transport{
					Proxy: http.ProxyFromEnvironment,
					DialContext: (&net.Dialer{
						Timeout:   5 * time.Second,
						KeepAlive: 3 * time.Second,
					}).DialContext,
					TLSClientConfig:     tlscfg,
					TLSHandshakeTimeout: 10 * time.Second,
					DisableKeepAlives:   false,
					MaxIdleConnsPerHost: 2,
					DisableCompression:  false,
				},
			}
		} else {
			client = &http.Client{
				Timeout: reqTimeout,
				Transport: &http.Transport{
					Proxy: http.ProxyFromEnvironment,
					DialContext: (&net.Dialer{
						Timeout:   5 * time.Second,
						KeepAlive: 3 * time.Second,
					}).DialContext,
					DisableKeepAlives:   false,
					MaxIdleConnsPerHost: 2,
					DisableCompression:  false,
				},
			}
		}
	}

	return client, nil
}

func NewAPIRequest(token string, reqURL string) (*http.Request, error) {
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "creating k8s api request")
	}
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("User-Agent", release.NAME+"/"+release.VERSION)
	return req, nil
}
