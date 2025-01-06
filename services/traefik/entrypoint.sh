#!/bin/sh

# NOTE: overload origin entrypoint

# first arg is `-f` or `--some-option`
if [ "${1#-}" != "$1" ]; then
    set -- traefik "$@"
fi

# if our command is a valid Traefik subcommand, let's invoke it through Traefik instead
# (this allows for "docker run traefik version", etc)
if traefik "$1" --help >/dev/null 2>&1
then
    set -- traefik "$@"
else
    echo "= '$1' is not a Traefik command: assuming shell execution." 1>&2
fi

# trigger config reload on certificate modification using providers.file.directory=/etc/traefik config
if [ "${1}" = "traefik" ]; then
    touch /etc/traefik/certs.log
    inotifywait /certs/proxy.crt -d -o /etc/traefik/certs.log -e modify --format '[%T] %|e %w%f' --timefmt '%F %T'
fi

exec "$@"
