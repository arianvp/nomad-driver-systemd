// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build linux

package main

import (
	log "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/plugins"
        "github.com/arianvp/nomad-driver-systemd/systemd"

)

func main() {
	plugins.Serve(factory)
}

func factory(log log.Logger) interface{} {
	return systemd.NewSingularityDriver(log)
}

