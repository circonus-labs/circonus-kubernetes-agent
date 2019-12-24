// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"testing"
)

func Test_validateAPIOptions(t *testing.T) {
	type args struct {
		apiKey     string
		apiKeyFile string
		apiApp     string
		apiURL     string
		apiCAFile  string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"invalid (no settings)", args{}, true},
		{"invalid (no key)", args{}, true},
		{"invalid (missing key file)", args{apiKeyFile: "testdata/missing"}, true},
		{"invalid (empty key file)", args{apiKeyFile: "testdata/empty_key"}, true},
		{"invalid (no app)", args{apiKeyFile: "testdata/valid_key"}, true},
		{"invalid (no url)", args{apiKey: "test-key", apiApp: "test-app"}, true},
		{"invalid url", args{apiKey: "test-key", apiApp: "test-app", apiURL: "foo"}, true},
		{"invalid url", args{apiKey: "test-key", apiApp: "test-app", apiURL: "foo_bar://herp/derp"}, true},
		{"invalid (missing ca file)", args{apiKey: "test-key", apiApp: "test-app", apiURL: "http://foo.com/bar", apiCAFile: "testdtaa/missing"}, true},
		{"valid", args{apiKey: "test-key", apiApp: "test-app", apiURL: "http://foo.com/bar"}, false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if err := validateAPIOptions(tt.args.apiKey, tt.args.apiKeyFile, tt.args.apiApp, tt.args.apiURL, tt.args.apiCAFile); (err != nil) != tt.wantErr {
				t.Errorf("validateAPIOptions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
