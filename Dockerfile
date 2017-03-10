FROM mhart/alpine-node:6.4.0

RUN apk update
RUN apk add bash

RUN mkdir /tmp/app
COPY ./WeatherApp/bin/ /tmp/app/ 
ADD ./docker-start.sh /tmp/app

USER root

EXPOSE 8099
ENTRYPOINT ["/tmp/app/docker-start.sh"]
