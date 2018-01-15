#!/bin/sh
set -e

fsdriver=$(grep -Eh -w -m1 "overlay|aufs" /proc/filesystems | cut -f2)
if [ $fsdriver == "overlay" ]; then
  fsdriver="overlay2"
fi

mkdir -p /etc/docker
/bin/cat > /etc/docker/daemon.json <<EOF
{
  "mtu": $(ip link show dev $(ip route |
                              awk '$1 == "default" { print $NF }') |
           awk '{for (i = 1; i <= NF; i++) if ($i == "mtu") print $(i+1)}')
}
EOF

dockerd-entrypoint.sh --storage-driver=$fsdriver &

# give docker a few seconds
sleep 3

exec "$@"
