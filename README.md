# `nomad-driver-systemd`
Allows scheduling of systemd units on a nomad cluster

## Example

```
$ sudo nomad agent -dev -plugin-dir "$PWD/plugins"
# Load the systemd unit onto systemd
$ cp webserver@.service /run/systemd/system
$ systemctl daemon-reload
$ nomad run example.hcl
==> Monitoring evaluation "2d88260e"
    Evaluation triggered by job "webserver"
    Allocation "156db366" created: node "e4557f90", group "webserver"
    Allocation "56342677" created: node "e4557f90", group "webserver"
    Allocation "83727afd" created: node "e4557f90", group "webserver"
    Allocation "9576f47e" created: node "e4557f90", group "webserver"
    Allocation "ec4c0d0d" created: node "e4557f90", group "webserver"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "2d88260e" finished with status "complete"

$ [arian@t490s:~]$ systemd-cgls --unit system-webserver.slice
Unit system-webserver.slice (/system.slice/system-webserver.slice):
├─webserver@156db366-e6e3-3819-812a-f778e0c93a24.service
│ └─4259 /usr/bin/python -m http.server 27434
├─webserver@ec4c0d0d-f3f3-38b1-7fdc-231437a5d1fa.service
│ └─4260 /usr/bin/python -m http.server 28809
├─webserver@56342677-4d23-8fe8-7c96-ae28774b803d.service
│ └─4261 /usr/bin/python -m http.server 23963
├─webserver@83727afd-362c-a2ce-b460-0267bf25c146.service
│ └─4258 /usr/bin/python -m http.server 21838
└─webserver@9576f47e-c429-ff8f-ae6b-fe679c120450.service
  └─4262 /usr/bin/python -m http.server 20124

```

## Configuring systemd
Systemd should be configured to uphold resource limits that nomad allocates for
the job.  If CPU limits are found, systemd should throttle the unit, and if
memory limits are reached, systemd should terminate the unit. Hence, this
plugin will implicitly enable `CPUAcocunting` and `MemoryAccounting` for the
units that it schedules. It will also implicitly set `LimitCPU` and
`LimitMemory` fields appropriately. Overriding these values is not recommended
for now. The user may set other limits on their units though.

It is important that systemd _never_ restarts units that are defined by the
plugin. Hence we will override each unit with a `RestartPolicy=never`. If a
systemd unit crashes, it's up to nomad to decide whether it should be restarted
on the same machine or re-scheduled on another machine. If the user sets
`RestartPolicy` we _will_ ignore it.

Features like `StateDirectory` should be used with care because we do not migrate
state when a unit is scheduled on a new machine.

Nomad will mount `/alloc`, `/local` and `/secrets` into the systemd unit.
Perhaps these in the future will be mounted under RuntimeDirectory=<unit_name>
but we haven't really decided yet.



## Upcoming features and TODOs

* Allow scheduling (and downloading?) of portable services
* Propagate nomad resource settings to  `systemd.resource-control` options
* Report resource utilization by querying the Cgroup tree


## Caveats

When a unit isn't parameterized only one will exist per box. that's probably not what you want.

https://www.nomadproject.io/docs/job-specification/restart.html might conflict with the
restart policy the unit itself has. Use either one, not both.

Logs are propagated to nomad, but not to journalctl
