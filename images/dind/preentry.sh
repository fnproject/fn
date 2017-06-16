#!/bin/sh
set -e

fsdriver=$(grep -Eh -w -m1 "overlay|aufs" /proc/filesystems | cut -f2)
if [ $fsdriver == "overlay" ]; then
  fsdriver="overlay2"
fi

dockerd-entrypoint.sh --storage-driver=$fsdriver &

# give docker a few seconds
sleep 3

exec "$@"
