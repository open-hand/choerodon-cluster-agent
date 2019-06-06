FROM golang:1.9.4-alpine3.7 as builder
WORKDIR /go/src/github.com/choerodon/choerodon-cluster-agent
COPY . .
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build .

FROM alpine:3.7
ARG KUBECTL_VRESION=v1.14.1
RUN apk --no-cache add \
        git \
        tini \
        curl \
        bash \
        tree \
        tzdata \
        openssh \
        ca-certificates && \
    wget -q -O /usr/bin/kubectl \
           "http://mirror.azure.cn/kubernetes/kubectl/${KUBECTL_VRESION}/bin/linux/amd64/kubectl" && \
    chmod a+x /usr/bin/kubectl && \
    ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone
COPY ./docker/ssh_config /etc/ssh/ssh_config
COPY --from=builder /go/src/github.com/choerodon/choerodon-cluster-agent/choerodon-cluster-agent /
ENTRYPOINT ["/sbin/tini", "--"]
CMD ["/choerodon-cluster-agent"]