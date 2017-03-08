FROM mhart/alpine-node:6.4.0

RUN apk update
RUN apk add bash

RUN mkdir /tmp/app
ADD bin/ /tmp/app/
ADD ./docker-start.sh /tmp/app

USER root

EXPOSE 8099
RUN cd /tmp/app
ENTRYPOINT ["/tmp/app/docker-start.sh"]
