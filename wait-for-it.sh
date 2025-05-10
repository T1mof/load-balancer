#!/bin/sh

set -e

if [ $# -eq 0 ]; then
    echo "Usage: $0 host:port [command args...]"
    exit 1
fi

host="$1"
shift

host_part=$(echo "$host" | cut -d: -f1)
port_part=$(echo "$host" | cut -d: -f2)

max_wait=60
wait_time=0

echo "Waiting for $host..."
while ! wget -q --spider "http://$host" 2>/dev/null; do
    wait_time=$((wait_time + 1))
    
    if [ $wait_time -ge $max_wait ]; then
        echo "Service at $host did not become available within $max_wait seconds - exiting"
        exit 1
    fi
    
    echo "Service at $host is unavailable - sleeping"
    sleep 1
done

echo "Service at $host is up!"

if [ $# -gt 0 ]; then
    exec "$@"
fi
