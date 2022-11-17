// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.19
// +build go1.19

package main

import (
	"runtime/debug"

	"github.com/circonus-labs/circonus-kubernetes-agent/cmd"
	_ "go.uber.org/automaxprocs"
)

func main() {
	debug.SetGCPercent(50)
	cmd.Execute()
}
