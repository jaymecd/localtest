FROM alpine:3.21

WORKDIR /app

RUN set -eu \
    && apk add --no-cache \
            dnsmasq \
            tini \
    && mkdir -p /etc/dnsmasq.d \
    && rm -rf /tmp/* /var/cache/apk/*

COPY ./entrypoint.sh /entrypoint.sh
COPY ./dnsmasq.conf /etc/dnsmasq.conf

HEALTHCHECK --interval=5s --timeout=1s --retries=3 \
    CMD netstat -nlt | grep -F :53 && netstat -nlu | grep -F :53

ENTRYPOINT ["/sbin/tini", "--", "/entrypoint.sh"]

CMD ["dnsmasq"]
