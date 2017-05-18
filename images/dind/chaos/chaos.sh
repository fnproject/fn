#!/bin/sh

set -ex

sleep 600 # 10 minutes

for i in 1..1000; do
  pkill -9 dockerd
  pkill -9 docker-containerd
  # remove pid file since we killed docker hard
  rm /var/run/docker.pid
  sleep 30
  docker daemon \
      --host=unix:///var/run/docker.sock \
      --host=tcp://0.0.0.0:2375 &
  sleep 300 # 5 minutes
done
