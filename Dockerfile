FROM golang:1.18-alpine
WORKDIR /zk
COPY bin/zk-redis-test .

ARG BUILD_NUMBER
ENV BUILD_NUMBER=$BUILD_NUMBER

CMD ["/zk/zk-redis-test", "-c", "/zk/config/config.yaml"]
