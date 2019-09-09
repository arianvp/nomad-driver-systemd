job "webserver" {
  datacenters = ["dc1"]
  type = "service"
  group "webserver" {
    count = 5
    task "webserver" {
      driver = "systemd"
      config {
        unit = "webserver@${NOMAD_ALLOC_ID}.service"
      }
      resources {
        network {
          port "http" {}
        }
      }
    }
  }
}
