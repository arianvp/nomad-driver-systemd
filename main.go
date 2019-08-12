// +build linux

package main

import (
	log "github.com/hashicorp/go-hclog"

	"github.com/arianvp/nomad-driver-systemd/systemd"
	"github.com/hashicorp/nomad/plugins"
)

func main() {
	plugins.Serve(factory)
}

func factory(log log.Logger) interface{} {
	return systemd.NewSystemdDriver(log)
}
