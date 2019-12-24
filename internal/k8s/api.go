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

	"github.com/pkg/errors"
)

func NewAPIClient(tlscfg *tls.Config) (*http.Client, error) {

	var client *http.Client

	if tlscfg != nil {
		client = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
					DualStack: true,
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
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
					DualStack: true,
				}).DialContext,
				DisableKeepAlives:   false,
				MaxIdleConnsPerHost: 2,
				DisableCompression:  false,
			},
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
	return req, nil
}
