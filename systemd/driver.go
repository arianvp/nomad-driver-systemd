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
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-systemd/dbus"
	"github.com/coreos/go-systemd/unit"
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
	unitsPath         = "/run/systemd/system"
)

func (d *Driver) getConn() (*dbus.Conn, error) {
	var err error
	d.once.Do(func() {
		var conn *dbus.Conn
		conn, err = dbus.New()
		if err != nil {
			return
		}
		d.conn = conn
	})
	if err != nil {
		return nil, err
	}
	return d.conn, nil
}

type Driver struct {
	// eventer is used to handle multiplexing of TaskEvents calls such that an
	// event can be broadcast to all callers
	eventer *eventer.Eventer

	// DBus connection to systemd
	// This will be null initially
	conn *dbus.Conn

	once sync.Once

	// config is the driver configuration set by the SetConfig RPC
	config *Config

	// nomadConfig is the client config from nomad
	nomadConfig *base.ClientDriverConfig

	// ctx is the context for the driver. It is passed to other subsystems to
	// coordinate shutdown
	ctx context.Context

	// cancel is called when the driver is shutting down and cancels the
	// ctx passed to any subsystems
	cancel context.CancelFunc

	// in memory mapping of taskIDs to taskHandles
	tasks taskStore

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

// why doesn't this get a ctx as an arugment? Who knows!!
func NewSystemdDriver(logger hclog.Logger) drivers.DriverPlugin {
	// question is;
	ctx, cancel := context.WithCancel(context.Background())
	// I think this is not needed
	logger = logger.Named(pluginName)

	return &Driver{
		eventer: eventer.NewEventer(ctx, logger),
		config:  &Config{},
		ctx:     ctx,
		cancel:  cancel,
		logger:  logger,
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
			// TODO this seems to make sense. If the caller context completes, we should complete
			d.cancel()
			return
		case <-d.ctx.Done():
			// If we're odne we're done, right?
			return
		case <-ticker.C:
			ticker.Reset(fingerprintPeriod)
			ch <- d.buildFingerprint()
		}
	}
}

// TODO do actual health check??
func (d *Driver) buildFingerprint() *drivers.Fingerprint {
	var health drivers.HealthState
	var desc string
	attrs := map[string]*pstructs.Attribute{}

	if d.config.Enabled {
		health = drivers.HealthStateHealthy
		desc = "ready"
	} else {
		health = drivers.HealthStateUndetected
	}

	return &drivers.Fingerprint{
		Attributes:        attrs,
		Health:            health,
		HealthDescription: desc,
	}
}

// TODO implement
func (d *Driver) RecoverTask(handle *drivers.TaskHandle) error {
	if handle == nil {
		return fmt.Errorf("error: handle cannot be nil")
	}
	if _, ok := d.tasks.Get(handle.Config.ID); ok {
		return nil
	}

	return nil
}

func taskConfigToUnitOptions(cfg *drivers.TaskConfig) []*unit.UnitOption {
	opts := []*unit.UnitOption{}
	//	ID              string
	//	JobName         string
	//	TaskGroupName   string
	//	Name            string
	//	Env             map[string]string
	env := strings.Join(cfg.EnvList(), " ")
	opts = append(opts, &unit.UnitOption{
		Section: "Service",
		Name:    "Environment",
		Value:   env,
	})
	//	DeviceEnv       map[string]string
	//	Resources       *Resources
	opts = append(opts, &unit.UnitOption{
		Section: "Service",
		Name:    "CPUShares",
		Value:   fmt.Sprintf("%d",cfg.Resources.LinuxResources.CPUShares),
	})
	opts = append(opts, &unit.UnitOption{
		Section: "Service",
		Name:    "MemoryLimit",
		Value:   fmt.Sprintf("%d",cfg.Resources.LinuxResources.MemoryLimitBytes),
	})
	//	Devices         []*DeviceConfig
	for _, device := range cfg.Devices {
		opts = append(opts, deviceToUnitOptions(device)...)
	}
	//	Mounts          []*MountConfig
	for _, mount := range cfg.Mounts {
		opts = append(opts, mountToUnitOption(mount))
	}
	//	User            string
	opts = append(opts, &unit.UnitOption{
		Section: "Service",
		Name:    "User",
		Value:   cfg.User,
	})
	//	AllocDir        string
	taskDir := cfg.TaskDir()
	taskDirMounts := []*drivers.MountConfig{
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
	}
	for _, mount := range taskDirMounts {
		opts = append(opts, mountToUnitOption(mount))
	}

	//	rawDriverConfig []byte
	//	StdoutPath      string
	opts = append(opts, &unit.UnitOption{
		Section: "Service",
		Name:    "StandardOutput",
		Value:   fmt.Sprintf("file:%s", cfg.StdoutPath),
	})
	//	StderrPath      string
	opts = append(opts, &unit.UnitOption{
		Section: "Service",
		Name:    "StandardError",
		Value:   fmt.Sprintf("file:%s", cfg.StderrPath),
	})
	//	AllocID         string

	return opts
}

func (d *Driver) StartTask(cfg *drivers.TaskConfig) (*drivers.TaskHandle, *drivers.DriverNetwork, error) {
	var driverConfig TaskConfig
	if err := cfg.DecodeDriverConfig(&driverConfig); err != nil {
		return nil, nil, fmt.Errorf("failed to decode driver config: %v", err)
	}
	// TODO we should check if this task already exists and crash if so
	handle := drivers.NewTaskHandle(0)
	handle.Config = cfg
	opts := taskConfigToUnitOptions(cfg)
	unitName := driverConfig.Unit
	unitContent := unit.Serialize(opts)
	dropinDir := path.Join(unitsPath, unitName+".d")
	err := os.MkdirAll(dropinDir, 0755)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to write unit file override: %s", err)
	}
	unitFile, err := os.Create(path.Join(dropinDir, "nomad.conf"))
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to write unit file override: %s", err)
	}
	_, err = io.Copy(unitFile, unitContent)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to write unit file override: %s", err)
	}
	conn, err := d.getConn()
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to connect to dbus: %s", err)
	}
	// TODO daemon-reload. maybe we don't need it
	// TODO perhaps we want to wait for the job to start up
	_, err = conn.StartUnit(unitName, "replace", nil)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to start unit: %s", err)
	}
	// keep around the handle
	subscription := conn.NewSubscriptionSet()
	subscription.Add(unitName)
	h := &taskHandle{handle: handle, subscription: subscription}
	d.tasks.Set(cfg.ID, h)
	return handle, nil, nil
}

func deviceToUnitOptions(device *drivers.DeviceConfig) []*unit.UnitOption {
	var opts []*unit.UnitOption
	opts = append(opts, &unit.UnitOption{
		Section: "Service",
		Name:    "BindPaths",
		Value:   fmt.Sprintf("%s:%s", device.HostPath, device.TaskPath),
	})
	opts = append(opts, &unit.UnitOption{
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
		bindPath = "BindReadOnlyPaths"
	}
	return &unit.UnitOption{
		Section: "Service",
		Name:    bindPath,
		Value:   fmt.Sprintf("%s:%s", mount.HostPath, mount.TaskPath),
	}
}

func (d *Driver) WaitTask(ctx context.Context, taskID string) (<-chan *drivers.ExitResult, error) {
	ch := make(chan *drivers.ExitResult)

	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	var driverConfig TaskConfig
	if err := handle.handle.Config.DecodeDriverConfig(&driverConfig); err != nil {
		return nil, fmt.Errorf("failed to decode driver config: %v", err)
	}
	unitStatus, errs := handle.subscription.Subscribe()

	go func() {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				ch <- &drivers.ExitResult{Err: ctx.Err()}
				return
			case err := <-errs:
				ch <- &drivers.ExitResult{Err: fmt.Errorf("Subscription error: %s", err)}
				return
			case statuses := <-unitStatus:
				status := statuses[driverConfig.Unit]
				if status == nil {
					// TODO only now; but in the future maybe multiple units can be present? IDK
					panic("should never happen by construction")
				}
				switch status.ActiveState {
				case "inactive":
					ch <- &drivers.ExitResult{}
					return
				case "failed":
					// TODO report a failed status
					ch <- &drivers.ExitResult{}
					return
				default:
					break
				}

			}
		}
	}()
	return ch, nil
}

func (d *Driver) StopTask(taskID string, timeout time.Duration, signal string) error {
	// We ignore the signal argument
	conn, err := d.getConn()
	if err != nil {
		return fmt.Errorf("Failed to connect to dbus: %v", err)
	}

	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return drivers.ErrTaskNotFound
	}
	var driverConfig TaskConfig
	if err := handle.handle.Config.DecodeDriverConfig(&driverConfig); err != nil {
		return fmt.Errorf("failed to decode driver config: %v", err)
	}

	// TODO StopUnit optionally takes a Channel that can be used for implementing WaitUnit
	conn.StopUnit(driverConfig.Unit, "replace", nil)

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

///    StartTask ->  RecoverTask ---/

func (d *Driver) DestroyTask(taskID string, force bool) error {
	// TODO portablectl detach

	// TODO check if container is running and check force
	d.tasks.Delete(taskID)
	return nil
}

func (d *Driver) InspectTask(taskID string) (*drivers.TaskStatus, error) {
	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}
	taskStatus := drivers.TaskStatus{}
	conn, err := d.getConn()
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to dbus: %v", err)
	}
	var driverConfig TaskConfig
	if err := handle.handle.Config.DecodeDriverConfig(&driverConfig); err != nil {
		return nil, fmt.Errorf("failed to decode driver config: %v", err)
	}
	statuses, err := conn.ListUnitsByNames([]string{driverConfig.Unit})
	if err := handle.handle.Config.DecodeDriverConfig(&driverConfig); err != nil {
		return nil, fmt.Errorf("failed to get unit status: %v", err)
	}
	status := statuses[0]

	taskStatus.Name = status.Name
	taskStatus.ID = taskID
	taskStatus.State = toTaskState(status.ActiveState)
	// TODO StartedAt, CompletedAt
	// TODO ExitResult

	return &taskStatus, nil
}

func toTaskState(activeState string) drivers.TaskState {
	switch activeState {
	case "activating":
		return drivers.TaskStateUnknown
	case "deactivating":
		return drivers.TaskStateUnknown
	case "failed":
		return drivers.TaskStateExited
	case "active":
		return drivers.TaskStateRunning
	case "inactive":
		return drivers.TaskStateExited
	default:
		return drivers.TaskStateUnknown
	}
}

func (d *Driver) TaskStats(ctx context.Context, taskID string, interval time.Duration) (<-chan *drivers.TaskResourceUsage, error) {
	// TODO query cgroup hierarchy for stats periodically.
	return nil, nil // fmt.Errorf("TaskStats")
}

func (d *Driver) TaskEvents(ctx context.Context) (<-chan *drivers.TaskEvent, error) {
	return d.eventer.TaskEvents(ctx)
}

func (d *Driver) SignalTask(taskID string, signal string) error {
	return fmt.Errorf("SignalTask not supported")
}

func (d *Driver) ExecTask(taskID string, cmd []string, timeout time.Duration) (*drivers.ExecTaskResult, error) {
	return nil, fmt.Errorf("ExecTask not supported")
}
