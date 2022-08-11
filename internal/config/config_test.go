// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"io"
	"testing"

	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config/keys"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func TestValidate(t *testing.T) {
	t.Log("Testing validate")

	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("no config")
	{
		err := Validate()
		if err != nil {
			t.Fatalf("Expected NO error, got (%s)", err)
		}
	}
}

func TestShowConfig(t *testing.T) {
	t.Log("Testing ShowConfig")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	t.Log("YAML")
	{
		viper.Set(keys.ShowConfig, "yaml")
		err := ShowConfig(io.Discard)
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}
	}

	t.Log("TOML")
	{
		viper.Set(keys.ShowConfig, "toml")
		err := ShowConfig(io.Discard)
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}
	}

	t.Log("JSON")
	{
		viper.Set(keys.ShowConfig, "json")
		err := ShowConfig(io.Discard)
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}
	}
}

func TestGetConfig(t *testing.T) {
	t.Log("Testing getConfig")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	cfg, err := getConfig()
	if err != nil {
		t.Fatalf("expected no error, got %s", err)
	}
	if cfg == nil {
		t.Fatal("expected not nil")
	}
}

func TestStatConfig(t *testing.T) {
	t.Log("Testing StatConfig")
	zerolog.SetGlobalLevel(zerolog.Disabled)

	err := StatConfig()
	if err != nil {
		t.Fatalf("expected no error, got %s", err)
	}
}
