// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package circonus contains methods for interfacing with circonus
package circonus

import (
	"strings"

	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config/keys"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

// apiLogshim is used to satisfy apiclient Logger interface for retryable-http (avoiding ptr receiver issue)
type apiLogshim struct {
	logh zerolog.Logger
}

func (l apiLogshim) Printf(fmt string, v ...interface{}) {
	if strings.HasPrefix(fmt, "[DEBUG]") {
		if !viper.GetBool(keys.APIDebug) {
			return
		}
	}

	l.logh.Printf(fmt, v...)
}

// submitLogshim is used to satisfy submission use of retryable-http Logger interface (avoiding ptr receiver issue)
type submitLogshim struct {
	logh zerolog.Logger
}

func (l submitLogshim) Printf(fmt string, v ...interface{}) {
	if strings.HasPrefix(fmt, "[DEBUG]") {
		if e := l.logh.Debug(); !e.Enabled() {
			return
		}
	}

	l.logh.Printf(fmt, v...)
}
