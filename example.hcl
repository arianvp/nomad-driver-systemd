job "webserver" {
  region = "us"
  datacenters = ["us-west-1", "us-east-1"]
  type = "service"
  group "webserver" {
    count = 10
    driver = "systemd"
    config {
      image = "local/nginx.raw"
      unit = "nginx@${NOMAD_ALLOC_INDEX}.service"
    }
    artifact {
      source = "https://example.com/nginx.raw"
      options {
        checksum = "sha256:12391239";
      };
    }
    resources {
      network {
        port "http" {}
      }
    }
  }
}
