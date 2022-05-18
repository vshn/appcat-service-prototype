FROM docker.io/library/alpine:3.15

RUN \
    apk add --no-cache bash curl

CMD [ "prom" ]

COPY prom /usr/local/bin/
