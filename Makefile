.PHONY: plugins/nomad-driver-systemd
plugins/nomad-driver-systemd:
	go build -o plugins/nomad-driver-systems


install:
	mkdir -p ${out}
	cp -r plugins ${out}

