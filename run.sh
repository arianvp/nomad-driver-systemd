systemd-run --user --slice cluster --unit consul consul agent -dev
systemd-run --user --slice cluster --unit nomad nomad agent -dev -plugin-dir "$PWD/plugins"
systemd-run --user --slice cluster --unit vault vault server -dev

echo "Type 'journalctl --user -xefu cluster.slice' to get logs"
echo "Type 'systemctl --user stop cluster.slice' to stop the dev cluster"

