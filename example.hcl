job "webserver" {
  datacenters = ["dc1"]
  type = "service"
  group "webserver" {
    count = 1
    task "systemd-portabled" {
      driver = "systemd"
      config {
        unit = "systemd-portabled.service"
      }
    }
  }
}
