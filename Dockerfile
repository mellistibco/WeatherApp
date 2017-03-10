FROM alpine:latest

RUN set -ex \
  && apk add --no-cache --virtual .build-deps \
     bash \
     gcc \
     musl-dev

RUN mkdir /tmp/app
ADD ./WeatherApp/bin/ /tmp/app/WeatherApp/bin
ADD ./docker-start.sh /tmp/app
RUN chmod 755 /tmp/app/WeatherApp/bin/WeatherApp

USER root

EXPOSE 8099
ENTRYPOINT ["/tmp/app/docker-start.sh"]
