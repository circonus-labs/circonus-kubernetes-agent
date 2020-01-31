// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package circonus contains methods for interfacing with circonus
package circonus

import (
	"strings"

	"github.com/rs/zerolog"
)

// logshim is used to satisfy apiclient Logger interface (avoiding ptr receiver issue)
type logshim struct {
	logh zerolog.Logger
}

func (l logshim) Printf(fmt string, v ...interface{}) {
	if strings.HasPrefix(fmt, "[DEBUG]") {
		if e := l.logh.Debug(); !e.Enabled() {
			return
		}
	}

	l.logh.Printf(fmt, v...)
}
