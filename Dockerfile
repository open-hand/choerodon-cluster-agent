FROM dockerhub.k8s.saas.hand-china.com/library/golang:1.13 as builder
WORKDIR /go/src/github.com/choerodon/choerodon-cluster-agent
COPY . .
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GO111MODULE=on go build -o ./choerodon-cluster-agent -mod=vendor -v ./cmd/manager

FROM registry.cn-shanghai.aliyuncs.com/c7n/debian:1.1

ENV USER_UID=33 \
    TINI_VERSION=v0.19.0

RUN mkdir -p /ssh-keys \
    && mkdir -p /polaris \
    && mkdir -p /tmp \
    && mkdir -p /etc/ssh \
    && mkdir -p /var/www \
    && mkdir -p /choerodon \
    && chown ${USER_UID}:0 /ssh-keys \
    && chown ${USER_UID}:0 /polaris \
    && chown ${USER_UID}:0 /tmp \
    && chown ${USER_UID}:0 /etc/ssh/ssh_config \
    && chown ${USER_UID}:0 /var/www \
    && chown ${USER_UID}:0 /choerodon

COPY --from=builder /go/src/github.com/choerodon/choerodon-cluster-agent/choerodon-cluster-agent /

COPY crd/ /choerodon

USER ${USER_UID}

ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["/choerodon-cluster-agent"]