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
	return systemd.NewSystemdDriver(log)
}

