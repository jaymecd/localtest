#!/bin/sh

# if first arg is `-f` or `--some-option`
# (this allows for "docker run dnsmasq --help")
if [ "${1#-}" != "$1" ]; then
    set -- dnsmasq "$@"
fi

if [ "${1}" != "dnsmasq" ]; then
    echo "DEBUG: '$1' is not a dnsmasq command: assuming shell execution. bye." 1>&2
    exec "$@"
fi

set -e

exec "$@"
