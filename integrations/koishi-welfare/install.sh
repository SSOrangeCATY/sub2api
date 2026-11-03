#!/bin/sh
# Install private transport and plugin files. Enable the plugin in Koishi after installation.
set -eu
test "$(id -u)" = 0
source_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
test -d /opt/koishi/data
install -d -m 0700 /opt/koishi/welfare-center /opt/koishi/data/welfare-center /opt/koishi/data/plugins/welfare-center /opt/koishi/data/plugins/welfare-center/assets
install -m 0600 "$source_dir/bridge.py" /opt/koishi/welfare-center/bridge.py
for filename in index.cjs report.cjs package.json welfare.test.cjs preview.cjs; do
  install -m 0600 "$source_dir/$filename" "/opt/koishi/data/plugins/welfare-center/$filename"
done
install -m 0600 "$source_dir/assets/Saira.ttf" "$source_dir/assets/Saira-OFL.txt" /opt/koishi/data/plugins/welfare-center/assets/
install -m 0644 "$source_dir/koishi-welfare-center.service" /etc/systemd/system/koishi-welfare-center.service
install -d -m 0700 /opt/koishi/data/node_modules
module=/opt/koishi/data/node_modules/koishi-plugin-welfare-center
if [ -e "$module" ] || [ -L "$module" ]; then
  test -L "$module"
  test "$(readlink "$module")" = ../plugins/welfare-center
else
  ln -s ../plugins/welfare-center "$module"
fi
systemctl daemon-reload
systemctl enable --now koishi-welfare-center.service
