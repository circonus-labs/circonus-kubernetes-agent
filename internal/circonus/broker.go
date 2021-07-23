// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package circonus

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/url"
	"strings"

	apiclient "github.com/circonus-labs/go-apiclient"
	"github.com/pkg/errors"
)

// initializeBroker fetches broker from circonus api and sets up broker tls config
func (c *Check) initializeBroker(client *apiclient.API, bundle *apiclient.CheckBundle) error {
	if client == nil {
		return errors.New("invalid state (nil api client)")
	}
	if bundle == nil {
		return errors.New("invalid state (nil bundle)")
	}
	if len(bundle.Brokers) == 0 {
		return errors.New("invalid bundle, 0 brokers")
	}

	if strings.Contains(c.submissionURL, "api.circonus.com") {
		return nil // api.circonus.com uses a public certificate, no tls config needed
	}

	cid := bundle.Brokers[0]
	broker, err := client.FetchBroker(apiclient.CIDType(&cid))
	if err != nil {
		return errors.Wrap(err, "fetching broker")
	}

	cn, err := brokerCN(broker, c.submissionURL)
	if err != nil {
		return errors.New("unable to determine broker CN")
	}

	if c.config.Check.BrokerCAFile != "" {
		cert, e := ioutil.ReadFile(c.config.Check.BrokerCAFile)
		if e != nil {
			return errors.Wrap(e, "configuring broker tls")
		}
		cp := x509.NewCertPool()
		if !cp.AppendCertsFromPEM(cert) {
			return errors.New("unable to add Broker CA Certificate to x509 cert pool")
		}
		c.brokerTLSConfig = &tls.Config{
			RootCAs:    cp,
			ServerName: cn,
			MinVersion: tls.VersionTLS12,
		}
		return nil
	}

	type cacert struct {
		Contents string `json:"contents"`
	}

	jsoncert, err := client.Get("/pki/ca.crt")
	if err != nil {
		return errors.Wrap(err, "fetching broker ca cert from api")
	}
	var cadata cacert
	if err := json.Unmarshal(jsoncert, &cadata); err != nil {
		return errors.Wrap(err, "parsing broker ca cert from api")
	}
	if cadata.Contents == "" {
		return errors.Errorf("unable to find ca cert 'Contents' attribute in api response (%+v)", cadata)
	}
	cp := x509.NewCertPool()
	if !cp.AppendCertsFromPEM([]byte(cadata.Contents)) {
		return errors.New("unable to add Broker CA Certificate to x509 cert pool")
	}
	c.brokerTLSConfig = &tls.Config{
		RootCAs:    cp,
		ServerName: cn,
		MinVersion: tls.VersionTLS12,
	}

	return nil
}

// brokerCN returns broker cn based on broker object
func brokerCN(broker *apiclient.Broker, submissionURL string) (string, error) {
	if broker == nil {
		return "", errors.New("invalid state (nil broker)")
	}
	u, err := url.Parse(submissionURL)
	if err != nil {
		return "", errors.Wrap(err, "determining broker cn")
	}

	hostParts := strings.Split(u.Host, ":")
	host := hostParts[0]

	if net.ParseIP(host) == nil { // it's a non-ip string
		return u.Host, nil
	}

	cn := ""

	for _, detail := range broker.Details {
		if detail.IP != nil && *detail.IP == host {
			cn = detail.CN
			break
		}
		if detail.ExternalHost != nil && *detail.ExternalHost == host {
			cn = detail.CN
			break
		}
	}

	if cn == "" {
		return "", errors.Errorf("error, unable to match URL host (%s) to Broker", u.Host)
	}

	return cn, nil
}
