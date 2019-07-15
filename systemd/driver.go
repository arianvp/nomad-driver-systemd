package systemd

import (
	"context"
	"fmt"
	"time"

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
		"enabled": hclspec.NewDefault(
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
		"singularity_cache": hclspec.NewAttr("singularity_cache", "string", false),
	})

	taskConfigSpec = hclspec.NewObject(map[string]*hclspec.Spec{
		"unit":   hclspec.NewAttr("unit", "string", true),
		"portable_service":   hclspec.NewAttr("portable_service", "string", true),


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

	AllowVolumes bool `codec:"volumes_enabled"`

	SingularityCache string `codec:"singularity_cache"`
}

// TaskConfig is the driver configuration of a task within a job
type TaskConfig struct {
	Unit string   `codec:"unit"`
	PortableService  []string `codec:"args"`
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

// PluginInfo return a base.PluginInfoResponse struct
func (d *Driver) PluginInfo() (*base.PluginInfoResponse, error) {
	return pluginInfo, nil
}

// ConfigSchema return a hclspec.Spec struct
func (d *Driver) ConfigSchema() (*hclspec.Spec, error) {
	return configSpec, nil
}

// SetConfig set the nomad agent config based on base.Config
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

// Shutdown the plugin
func (d *Driver) Shutdown(ctx context.Context) error {
	d.signalShutdown()
	return nil
}

// TaskConfigSchema returns a hclspec.Spec struct
func (d *Driver) TaskConfigSchema() (*hclspec.Spec, error) {
	return taskConfigSpec, nil
}

// Capabilities a drivers.Capabilities struct
func (d *Driver) Capabilities() (*drivers.Capabilities, error) {
	return capabilities, nil
}

// Fingerprint return the plugin fingerprint
func (d *Driver) Fingerprint(ctx context.Context) (<-chan *drivers.Fingerprint, error) {
	return nil, nil
}


// RecoverTask try to recover a failed task, if not return error
func (d *Driver) RecoverTask(handle *drivers.TaskHandle) error {
	return nil
}

// StartTask setup the task exec and calls the container excecutor
func (d *Driver) StartTask(cfg *drivers.TaskConfig) (*drivers.TaskHandle, *drivers.DriverNetwork, error) {
	return nil, nil, nil
}

// WaitTask watis for task completion
func (d *Driver) WaitTask(ctx context.Context, taskID string) (<-chan *drivers.ExitResult, error) {
        return nil, nil
}

// StopTask shutdown a tasked based on its taskID
func (d *Driver) StopTask(taskID string, timeout time.Duration, signal string) error {
	return nil
}

// DestroyTask delete task
func (d *Driver) DestroyTask(taskID string, force bool) error {
	return nil
}

// InspectTask retrieves task info
func (d *Driver) InspectTask(taskID string) (*drivers.TaskStatus, error) {
	return nil, nil
}

// TaskStats get task stats
func (d *Driver) TaskStats(ctx context.Context, taskID string, interval time.Duration) (<-chan *drivers.TaskResourceUsage, error) {
	return nil,nil
}

// TaskEvents return a chan *drivers.TaskEvent
func (d *Driver) TaskEvents(ctx context.Context) (<-chan *drivers.TaskEvent, error) {
    return nil,nil
}

// SignalTask send a specific signal to a taskID
func (d *Driver) SignalTask(taskID string, signal string) error {
	return fmt.Errorf("Singularity driver does not support signals")
}

// ExecTask calls a exec cmd over a running task
func (d *Driver) ExecTask(taskID string, cmd []string, timeout time.Duration) (*drivers.ExecTaskResult, error) {
	return nil, fmt.Errorf("Singularity driver does not support exec") //TODO
}
