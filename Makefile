.PHONY: plugins/nomad-systemd-driver
plugins/nomad-systemd-driver:
	go build -o plugins/nomad-systemd-driver


install:
	mkdir -p ${out}
	cp -r plugins ${out}

