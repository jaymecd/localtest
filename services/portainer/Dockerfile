ARG PORTAINER_VERSION=configured_via_composer

FROM portainer/portainer-ce:${PORTAINER_VERSION}-alpine

RUN apk add --no-cache curl jq tini

COPY ./entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/sbin/tini", "--", "/entrypoint.sh"]

CMD ["/portainer"]
