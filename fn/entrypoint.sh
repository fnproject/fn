#!/bin/sh

HOST=$(/sbin/ip route|awk '/default/ { print $3 }')

echo "$HOST default localhost localhost.local" > /etc/hosts

/fn "$@"