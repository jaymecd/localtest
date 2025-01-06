#!/bin/sh

# NOTE: alternated origin entrypoint

# if first arg is `-f` or `--some-option`
# (this allows for "docker run traefik --help")
if [ "${1#-}" != "$1" ]; then
    set -- traefik "$@"
# or command is a valid Traefik subcommand, let's invoke it through Traefik instead
# (this allows for "docker run traefik version" & etc)
elif [ "${1}" != "traefik" ] && traefik "$1" --help >/dev/null 2>&1; then
    set -- traefik "$@"
fi

if [ "${1}" != "traefik" ]; then
    echo "DEBUG: '$1' is not a traefik command: assuming shell execution. bye." 1>&2
    exec "$@"
fi

set -e

# update static config as it's not allowed to both at once
sed -i \
    -e "s/\${TRAEFIK_LOG_LEVEL}/${TRAEFIK_LOG_LEVEL:-INFO}/g" \
    /etc/traefik/traefik.yml

if grep -qF '${TRAEFIK_' /etc/traefik/traefik.yml; then
    echo "Error: unhandled TRAEFIK_* placeholder found in traefik.yml file" 1>&2
    false
fi

# trigger config reload on certificate modification using providers.file.directory=/etc/traefik config
echo "Watching /certs/proxy.crt for changes to reload Traefik config ..."
touch /etc/traefik/certs.log
inotifywait /certs/proxy.crt -d -q -o /etc/traefik/certs.log -e modify --format '[%T] %|e %w%f' --timefmt '%F %T'

exec "$@"
