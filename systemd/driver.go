package systemd
// Some reading material
//  * https://systemd.io/CGROUP_DELEGATION.html
//  * https://events.static.linuxfound.org/sites/events/files/slides/cgroup_and_namespaces.pdf
// Specifically, this note:
// ⚡ Currently, the algorithm for mapping between slice/scope/service unit
// naming and their cgroup paths is not considered public API of systemd, and
// may change in future versions. This means: it’s best to avoid implementing a
// local logic of translating cgroup paths to slice/scope/service names in your
// program, or vice versa — it’s likely going to break sooner or later. Use the
// appropriate D-Bus API calls for that instead, so that systemd translates
// this for you. (Specifically: each Unit object has a ControlGroup property to
// get the cgroup for a unit. The method GetUnitByControlGroup() may be used to
// get the unit for a cgroup.)

import (
	"context"
	// "fmt"
	"time"

	unit "github.com/coreos/go-systemd/unit"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/drivers/shared/eventer"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	// 	pstructs "github.com/hashicorp/nomad/plugins/shared/structs"
)

const (
	pluginName = "systemd"
)

var (
	// pluginInfo is the response returned for the PluginInfo RPC
	pluginInfo = &base.PluginInfoResponse{
		Type:              base.PluginTypeDriver,
		PluginApiVersions: []string{"0.1.0"},
		PluginVersion:     "0.0.1",
		Name:              pluginName,
	}

	// configSpec is the hcl specification returned by the ConfigSchema RPC
	configSpec = hclspec.NewObject(map[string]*hclspec.Spec{
		/*"enabled": hclspec.NewDefault(
			hclspec.NewAttr("enabled", "bool", false),
			hclspec.NewLiteral("true"),
		),
		"no_cgroups": hclspec.NewDefault(
			hclspec.NewAttr("no_cgroups", "bool", false),
			hclspec.NewLiteral("false"),
		),
		"volumes_enabled": hclspec.NewDefault(
			hclspec.NewAttr("volumes_enabled", "bool", false),
			hclspec.NewLiteral("true"),
		),
		"singularity_cache": hclspec.NewAttr("singularity_cache", "string", false),*/
	})

	taskConfigSpec = hclspec.NewObject(map[string]*hclspec.Spec{
		"portable_service": hclspec.NewAttr("portable_service", "string", true),
	})

	// capabilities is returned by the Capabilities RPC and indicates what
	// optional features this driver supports
	capabilities = &drivers.Capabilities{
		SendSignals: true,
		Exec:        true,
		FSIsolation: drivers.FSIsolationChroot,
	}
)

type Driver struct {
	// eventer is used to handle multiplexing of TaskEvents calls such that an
	// event can be broadcast to all callers
	eventer *eventer.Eventer

	// config is the driver configuration set by the SetConfig RPC
	config *Config

	// nomadConfig is the client config from nomad
	nomadConfig *base.ClientDriverConfig

	// ctx is the context for the driver. It is passed to other subsystems to
	// coordinate shutdown
	ctx context.Context

	// signalShutdown is called when the driver is shutting down and cancels the
	// ctx passed to any subsystems
	signalShutdown context.CancelFunc

	// logger will log to the Nomad agent
	logger hclog.Logger
}

// Config is the driver configuration set by the SetConfig RPC call
type Config struct {
	Enabled bool `codec:"enabled"`
}

// TaskConfig is the driver configuration of a task within a job
type TaskConfig struct {
	Unit            string   `codec:"unit"`
	PortableService []string `codec:"args"`
}

// NewSingularityDriver returns a new DriverPlugin implementation
func NewSystemdDriver(logger hclog.Logger) drivers.DriverPlugin {
	ctx, cancel := context.WithCancel(context.Background())
	logger = logger.Named(pluginName)

	return &Driver{
		eventer:        eventer.NewEventer(ctx, logger),
		config:         &Config{},
		ctx:            ctx,
		signalShutdown: cancel,
		logger:         logger,
	}
}

func (d *Driver) PluginInfo() (*base.PluginInfoResponse, error) {
	return pluginInfo, nil
}

func (d *Driver) ConfigSchema() (*hclspec.Spec, error) {
	return configSpec, nil
}

func (d *Driver) SetConfig(cfg *base.Config) error {
	var config Config
	if len(cfg.PluginConfig) != 0 {
		if err := base.MsgPackDecode(cfg.PluginConfig, &config); err != nil {
			return err
		}
	}

	d.config = &config
	if cfg.AgentConfig != nil {
		d.nomadConfig = cfg.AgentConfig.Driver
	}

	return nil
}

func (d *Driver) Shutdown(ctx context.Context) error {
	d.signalShutdown()
	return nil
}

func (d *Driver) TaskConfigSchema() (*hclspec.Spec, error) {
	return nil, nil
}

func (d *Driver) Capabilities() (*drivers.Capabilities, error) {
	return nil, nil
}

func (d *Driver) Fingerprint(ctx context.Context) (<-chan *drivers.Fingerprint, error) {
        // TODO Send an initial fingerprint
        // TODO send periodic fingerprints
	return nil, nil
}

func (d *Driver) RecoverTask(handle *drivers.TaskHandle) error {
        // TODO systemctl status <task-id>
	return nil
}

func (d *Driver) StartTask(cfg *drivers.TaskConfig) (*drivers.TaskHandle, *drivers.DriverNetwork, error) {
	// portablectl attach --runtime  <name>
	var opts []*unit.UnitOption
	append(opts,
		&unit.UnitOption{
			Section: "Service",
			Name:    "BindPaths",
			Value:   "/alloc:TODO",
		},
		&unit.UnitOption{
			Section: "Service",
			Name:    "BindPaths",
			Value:   "/local:TODO",
		},
		&unit.UnitOption{
			Section: "Service",
			Name:    "BindPaths",
			Value:   "/secrets:TODO",
		},
	)

        // TODO: Wait for answer on https://groups.google.com/forum/#!topic/nomad-tool/eegWfX2zngw
        // because the nomad code is a bit confusing in this regard

        // or whatever the go syntax is, fucking pscho
        for _, mount : range(cfg.Mounts) {
          append(opts, &unit.UnitOption {
            Section: "Service",
            Name: "BindPaths",
            Value:  "TODO",
          })
        }
        for _, device : range(cfg.Devices) {
            // TODO BindPaths
            // TODO DeviceAllow=rwm
        }

	// TODO systemctl start <name>.service
	return nil, nil, nil
}

func (d *Driver) WaitTask(ctx context.Context, taskID string) (<-chan *drivers.ExitResult, error) {
	return nil, nil
}

func (d *Driver) StopTask(taskID string, timeout time.Duration, signal string) error {
        // TODO systemctl set-property KillTimeoutOrSoemthing timeout
        // TODO systemctl stop
	return nil
}

func (d *Driver) DestroyTask(taskID string, force bool) error {
        // TODO portablectl detach
	return nil
}

func (d *Driver) InspectTask(taskID string) (*drivers.TaskStatus, error) {
        // TODO systemctl show
	return nil, nil
}

func (d *Driver) TaskStats(ctx context.Context, taskID string, interval time.Duration) (<-chan *drivers.TaskResourceUsage, error) {
        // TODO query cgroup hierarchy for stats periodically.
	return nil, nil
}

func (d *Driver) TaskEvents(ctx context.Context) (<-chan *drivers.TaskEvent, error) {
        // TODO
	return nil, nil
}

func (d *Driver) SignalTask(taskID string, signal string) error {
        // TODO systemctl kill -s
	return nil
}

func (d *Driver) ExecTask(taskID string, cmd []string, timeout time.Duration) (*drivers.ExecTaskResult, error) {
	return nil, nil
}
