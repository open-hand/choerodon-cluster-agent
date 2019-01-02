FROM golang:1.9.4-alpine3.7 as builder
WORKDIR /go/src/github.com/choerodon/choerodon-cluster-agent
COPY . .
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build .

FROM registry.cn-hangzhou.aliyuncs.com/choerodon-tools/agent-kubectl:1.9.0
WORKDIR /

RUN apk --no-cache add \
  git \
  openssh
RUN apk update && apk add curl bash tree tzdata \
    && cp -r -f /usr/share/zoneinfo/Hongkong /etc/localtime \
    && echo -ne "Alpine Linux 3.4 image. (`uname -rsv`)\n" >> /root/.built
RUN apk add --no-cache tini

COPY ./docker/ssh_config /etc/ssh/ssh_config
COPY --from=builder /go/src/github.com/choerodon/choerodon-cluster-agent/choerodon-cluster-agent .
ENTRYPOINT ["/sbin/tini", "--"]
CMD ["/choerodon-cluster-agent"]
