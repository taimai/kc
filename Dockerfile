FROM golang:1.8
MAINTAINER team@your.domain.com
RUN mkdir /app
WORKDIR /app
ADD . /app
CMD make run
