FROM traefik:v3.2.2

RUN apk add --no-cache inotify-tools

COPY ./entrypoint.sh /entrypoint.sh
COPY ./traefik.yml /etc/traefik/traefik.yml

ENTRYPOINT ["/entrypoint.sh"]

CMD ["traefik"]
