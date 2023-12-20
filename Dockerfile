FROM golang:1.18-alpine
WORKDIR /zk
COPY bin/zk-db-test .

ARG BUILD_NUMBER
ENV BUILD_NUMBER=$BUILD_NUMBER

CMD ["/zk/zk-db-test", "-c", "/zk/config/config.yaml"]
