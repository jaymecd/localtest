FROM alpine:3.21

WORKDIR /app

RUN echo "@testing https://dl-cdn.alpinelinux.org/alpine/edge/testing" >> /etc/apk/repositories \
        && apk add --no-cache bash openssl ca-certificates mkcert@testing

COPY ./entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]

CMD ["generate-certificates"]
