// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package circonus

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	apiclient "github.com/circonus-labs/go-apiclient"
)

// initializeBroker fetches broker from circonus api and sets up broker tls config
func (c *Check) initializeBroker(client *apiclient.API, bundle *apiclient.CheckBundle) error {
	if client == nil {
		return fmt.Errorf("invalid state (nil api client)")
	}
	if bundle == nil {
		return fmt.Errorf("invalid state (nil bundle)")
	}
	if len(bundle.Brokers) == 0 {
		return fmt.Errorf("invalid bundle, 0 brokers")
	}

	if strings.Contains(c.submissionURL, "api.circonus.com") {
		return nil // api.circonus.com uses a public certificate, no tls config needed
	}

	cid := bundle.Brokers[0]
	broker, err := client.FetchBroker(apiclient.CIDType(&cid))
	if err != nil {
		return fmt.Errorf("fetching broker: %w", err)
	}

	cn, cnList, err := getBrokerCNList(broker, c.submissionURL)
	if err != nil {
		return err
	}

	data, err := getCACert(c.config.Check.BrokerCAFile, client)
	if err != nil {
		return err
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(data) {
		return fmt.Errorf("unable to add Broker CA Certificate to x509 cert pool")
	}
	c.brokerTLSConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: cn,
		// go1.15+ see VerifyConnection below - until CN added to SAN in broker certs
		// NOTE: InsecureSkipVerify:true does NOT disable VerifyConnection()
		InsecureSkipVerify: true, //nolint:gosec
		VerifyConnection: func(cs tls.ConnectionState) error {
			commonName := cs.PeerCertificates[0].Subject.CommonName
			// if commonName != cs.ServerName {
			if !strings.Contains(cnList, commonName) {
				c.log.Error().Str("cn", commonName).Str("cn-list", cnList).Msg("unable to match cert subject cn to broker cn-list")
				return x509.CertificateInvalidError{
					Cert:   cs.PeerCertificates[0],
					Reason: x509.NameMismatch,
					Detail: fmt.Sprintf("cn: %q, acceptable: %q", commonName, cnList),
				}
			}
			opts := x509.VerifyOptions{
				Roots:         certPool,
				Intermediates: x509.NewCertPool(),
			}
			for _, cert := range cs.PeerCertificates[1:] {
				opts.Intermediates.AddCert(cert)
			}
			_, err := cs.PeerCertificates[0].Verify(opts)
			if err != nil {
				return fmt.Errorf("peer cert verify: %w", err)
			}
			return nil
		},
	}

	return nil
}

// getCACert will read from a file or fetch from API
func getCACert(fn string, client *apiclient.API) ([]byte, error) {
	if fn != "" {
		data, err := os.ReadFile(fn)
		if err != nil {
			return nil, fmt.Errorf("configuring broker tls: %w", err)
		}

		return data, nil
	}

	type cacert struct {
		Contents string `json:"contents"`
	}

	jsoncert, err := client.Get("/pki/ca.crt")
	if err != nil {
		return nil, fmt.Errorf("fetching broker ca cert from api: %w", err)
	}
	var cadata cacert
	if err := json.Unmarshal(jsoncert, &cadata); err != nil {
		return nil, fmt.Errorf("parsing broker ca cert from api: %w", err)
	}
	if cadata.Contents == "" {
		return nil, fmt.Errorf("unable to find ca cert 'Contents' attribute in api response (%+v)", cadata)
	}

	return []byte(cadata.Contents), nil
}

// getBrokerCNList returns broker cn based on broker object
func getBrokerCNList(broker *apiclient.Broker, submissionURL string) (string, string, error) {
	if broker == nil {
		return "", "", fmt.Errorf("invalid state (nil broker)")
	}
	u, err := url.Parse(submissionURL)
	if err != nil {
		return "", "", fmt.Errorf("determining broker cn: %w", err)
	}

	hostParts := strings.Split(u.Host, ":")
	host := hostParts[0]

	if net.ParseIP(host) == nil { // it's a non-ip string
		return u.Host, u.Host, nil
	}

	cn := ""
	cnList := make([]string, 0, len(broker.Details))
	for _, detail := range broker.Details {
		if detail.IP != nil && *detail.IP == host {
			cn = detail.CN
			cnList = append(cnList, detail.CN)
		} else if detail.ExternalHost != nil && *detail.ExternalHost == host {
			cn = detail.CN
			cnList = append(cnList, detail.CN)
		}
	}

	if len(cnList) == 0 {
		return "", "", fmt.Errorf("unable to match URL host (%s) to broker instance", u.Host)
	}

	return cn, strings.Join(cnList, ","), nil
}
