ARG TRAEFIK_VERSION=configured_via_composer

FROM traefik:${TRAEFIK_VERSION}

RUN apk add --no-cache inotify-tools tini

COPY ./entrypoint.sh /entrypoint.sh
COPY ./traefik.yml /etc/traefik/traefik.yml

HEALTHCHECK --interval=5s --timeout=1s --retries=3 \
    CMD traefik healthcheck

ENTRYPOINT ["/sbin/tini", "--", "/entrypoint.sh"]

CMD ["traefik"]
