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
	"fmt"
	"time"
        "sync"

	unit "github.com/coreos/go-systemd/unit"
	dbus "github.com/coreos/go-systemd/dbus"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/drivers/shared/eventer"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	pstructs "github.com/hashicorp/nomad/plugins/shared/structs"
)

const (
	pluginName        = "systemd"
	fingerprintPeriod = 30 * time.Second
)

// singletons! yuck! but I'm not sure how else to accomplish this shared global
// piece of state
var (
        once sync.Once
	conn *dbus.Conn
)

// TODO close the dbus connection on exit
func getConn() (*dbus.Conn, error) {
        var err error
	once.Do(func() {
	        conn, err = dbus.New()
	})
        if err != nil {
          return nil, err
        }
	return conn, nil
}

type Driver struct {
	// eventer is used to handle multiplexing of TaskEvents calls such that an
	// event can be broadcast to all callers
	eventer *eventer.Eventer

        // DBus connection to systemd
        // This will be null initially
        conn *dbus.Conn

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
	Unit string `codec:"unit"`
}

type TaskState struct {
    conn dbus.Conn
}



// why doesn't this get a ctx as an arugment? Who knows!!
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
	return &base.PluginInfoResponse{
		Type:              base.PluginTypeDriver,
		PluginApiVersions: []string{"0.1.0"},
		PluginVersion:     "0.0.1",
		Name:              pluginName,
	}, nil
}

func (d *Driver) ConfigSchema() (*hclspec.Spec, error) {
	return hclspec.NewObject(map[string]*hclspec.Spec{
		"enabled": hclspec.NewDefault(
			hclspec.NewAttr("enabled", "bool", false),
			hclspec.NewLiteral("true"),
		),
	}), nil
}

// This is marked as internal, but seems to be the only way to cancel our Context?
// TODO: ask about this on the mailing list?
func (d *Driver) Shutdown() {
    d.signalShutdown()
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

func (d *Driver) TaskConfigSchema() (*hclspec.Spec, error) {
	return hclspec.NewObject(map[string]*hclspec.Spec{
		"unit": hclspec.NewAttr("unit", "string", true),
	}), nil
}

func (d *Driver) Capabilities() (*drivers.Capabilities, error) {
	return &drivers.Capabilities{
		SendSignals: true,
		Exec:        false, // TODO: can probably implement
		FSIsolation: drivers.FSIsolationChroot,
	}, nil
}

func (d *Driver) Fingerprint(ctx context.Context) (<-chan *drivers.Fingerprint, error) {
	ch := make(chan *drivers.Fingerprint)
	go d.handleFingerprint(ctx, ch)
	return ch, nil
}

func (d *Driver) handleFingerprint(ctx context.Context, ch chan<- *drivers.Fingerprint) {
	defer close(ch)
	ticker := time.NewTimer(0)
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			ticker.Reset(fingerprintPeriod)
			ch <- d.buildFingerprint()
		}
	}
}

func (d *Driver) buildFingerprint() *drivers.Fingerprint {
	var health drivers.HealthState
	var desc string
	attrs := map[string]*pstructs.Attribute{}

	if d.config.Enabled {
		health = drivers.HealthStateHealthy
		desc = "ready"
		// attrs["driver.lxc"] = pstructs.NewBoolAttribute(true)
		// ttrs["driver.lxc.version"] = pstructs.NewStringAttribute(lxcVersion)
	} else {
		health = drivers.HealthStateUndetected
	}

	return &drivers.Fingerprint{
		Attributes:        attrs,
		Health:            health,
		HealthDescription: desc,
	}
}

func (d *Driver) RecoverTask(handle *drivers.TaskHandle) error {
	// TODO systemctl status <task-id>
	return nil
}


func (d *Driver) StartTask(cfg *drivers.TaskConfig) (*drivers.TaskHandle, *drivers.DriverNetwork, error) {
	var driverConfig TaskConfig
	if err := cfg.DecodeDriverConfig(&driverConfig); err != nil {
		return nil, nil, fmt.Errorf("failed to decode driver config: %v", err)
	}

	// TODO we should check if this task already exists
	handle := drivers.NewTaskHandle(0)
	handle.Config = cfg

	conn, err := getConn()
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to connect to dbus: %s", err)
	}

	taskDir := cfg.TaskDir()
	mounts := cfg.Mounts
	devices := cfg.Devices

	mounts = append(mounts,
		&drivers.MountConfig{
			HostPath: taskDir.LocalDir,
			TaskPath: "/local",
		},
		&drivers.MountConfig{
			HostPath: taskDir.SharedAllocDir,
			TaskPath: "/alloc",
		},
		&drivers.MountConfig{
			HostPath: taskDir.SecretsDir,
			TaskPath: "/secrets",
		},
	)

        opts := []*unit.UnitOption{}
	for _, mount := range mounts {
		opts = append(opts, mountToUnitOption(mount))
	}
	for _, device := range devices {
		opts = append(opts, deviceToUnitOptions(device)...)
	}

        // TODO perhaps we want to wait for the job to start up
        _, err = conn.StartUnit(driverConfig.Unit, "replace", nil)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to start unit: %s", err)
	}


	// TODO write

	// TODO portablectl attach --runtime  <name>
	// TODO write the drop-in unit to disk and systemctl daemon-reload

	// TODO systemctl start <name>.service

	return handle, nil, nil
}

func deviceToUnitOptions(device *drivers.DeviceConfig) []*unit.UnitOption {
	var opts []*unit.UnitOption
        opts = append(opts, &unit.UnitOption {
		Section: "Service",
		Name:    "BindPaths",
		Value:   fmt.Sprintf("%s:%s", device.HostPath, device.TaskPath),
        })
        opts = append(opts, &unit.UnitOption {
		Section: "Service",
		Name:    "DeviceAllow",
		Value:   fmt.Sprintf("%s %s", device.HostPath, device.Permissions),
        })
	return nil
}
func mountToUnitOption(mount *drivers.MountConfig) *unit.UnitOption {
	var bindPath string
	if mount.Readonly {
		bindPath = "BindPaths"
	} else {
		bindPath = "ReadOnlyBindPaths"
	}
	return &unit.UnitOption{
		Section: "Service",
		Name:    bindPath,
		Value:   fmt.Sprintf("%s:%s", mount.HostPath, mount.TaskPath),
	}
}

func (d *Driver) WaitTask(ctx context.Context, taskID string) (<-chan *drivers.ExitResult, error) {
        
	return nil, fmt.Errorf("nooo")
}

func (d *Driver) StopTask(taskID string, timeout time.Duration, signal string) error {
	// TODO systemctl set-property KillTimeoutOrSoemthing timeout
	// TODO systemctl stop
        // TODO what to do when it doesn't stop?
        // TODO store task id somewhere
	/* conn, err := getConn()
	if err != nil {
		fmt.Errorf("Failed to connect to dbus: %s", err)
	} */

        d.logger.Debug("Success")
	return nil

}

func (d *Driver) DestroyTask(taskID string, force bool) error {
	// TODO portablectl detach
	return nil
}

func (d *Driver) InspectTask(taskID string) (*drivers.TaskStatus, error) {
	// TODO systemctl show
	return nil, fmt.Errorf("InspectTask")
}

func (d *Driver) TaskStats(ctx context.Context, taskID string, interval time.Duration) (<-chan *drivers.TaskResourceUsage, error) {
	// TODO query cgroup hierarchy for stats periodically.
	return nil, nil // fmt.Errorf("TaskStats")
}

func (d *Driver) TaskEvents(ctx context.Context) (<-chan *drivers.TaskEvent, error) {
        return d.eventer.TaskEvents(ctx)
}

func (d *Driver) SignalTask(taskID string, signal string) error {
	// TODO systemctl kill -s
	return fmt.Errorf("SignalTask")
}

func (d *Driver) ExecTask(taskID string, cmd []string, timeout time.Duration) (*drivers.ExecTaskResult, error) {
	return nil, fmt.Errorf("ExecTask")
}
