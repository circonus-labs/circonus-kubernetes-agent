// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"io/ioutil"
	"net/url"

	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config/defaults"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config/keys"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

func validateAPIOptions(apiKey, apiKeyFile, apiApp, apiURL, apiCAFile string) error {
	if apiKeyFile == "" && apiKey == "" {
		return errors.New("API key is required")
	}

	if apiApp == "" {
		return errors.New("API app is required")
	}

	if apiURL == "" {
		return errors.New("API URL is required")
	}

	if apiKeyFile != "" && apiKey == "" {
		f, err := verifyFile(apiKeyFile)
		if err != nil {
			return err
		}
		data, err := ioutil.ReadFile(f)
		if err != nil {
			return err
		}
		apiKey = string(data)
		if apiKey == "" {
			return errors.Errorf("invalid API key file (%s), empty key", apiKeyFile)
		}
	}

	if apiURL != defaults.APIURL {
		parsedURL, err := url.Parse(apiURL)
		if err != nil {
			return errors.Wrap(err, "invalid API URL")
		}
		if parsedURL.Scheme == "" || parsedURL.Host == "" || parsedURL.Path == "" {
			return errors.Errorf("invalid API URL (%s)", apiURL)
		}
	}

	// NOTE the api ca file doesn't come from the cosi config
	if apiCAFile != "" {
		f, err := verifyFile(apiCAFile)
		if err != nil {
			return err
		}
		viper.Set(keys.APICAFile, f)
	}

	viper.Set(keys.APITokenKey, apiKey)
	viper.Set(keys.APITokenApp, apiApp)
	viper.Set(keys.APIURL, apiURL)

	return nil
}
