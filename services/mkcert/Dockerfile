FROM alpine:3.21

WORKDIR /app

RUN set -eu \
    && echo "@testing https://dl-cdn.alpinelinux.org/alpine/edge/testing" >> /etc/apk/repositories \
    && apk add --no-cache \
            bash \
            ca-certificates \
            mkcert@testing \
            openssl \
    && rm -rf /tmp/* /var/cache/apk/*

COPY ./generate_certs.sh /generate_certs.sh

CMD ["/generate_certs.sh"]
