FROM dockerhub.k8s.saas.hand-china.com/library/golang:1.13 as builder
WORKDIR /go/src/github.com/choerodon/choerodon-cluster-agent
COPY . .
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GO111MODULE=on go build -o ./choerodon-cluster-agent -mod=vendor -v ./cmd/manager

FROM registry.hand-china.com/hzero-c7ncd/debian:1.0

ENV USER_UID=33 \
    TINI_VERSION=v0.19.0

RUN echo "${USER_NAME}:x:${USER_UID}:0:${USER_NAME} user:${HOME}:/sbin/nologin" >> /etc/passwd \
    && mkdir -p ${HOME} \
    && mkdir -p /ssh-keys \
    && mkdir -p /polaris \
    && mkdir -p /tmp \
    && mkdir -p /etc/ssh \
    && mkdir -p /root/.kube \
    && chown ${USER_UID}:0 ${HOME} \
    && chown ${USER_UID}:0 /ssh-keys \
    && chown ${USER_UID}:0 /polaris \
    && chown ${USER_UID}:0 /tmp \
    && chown ${USER_UID}:0 /etc/ssh/ssh_config \
    && chown ${USER_UID}:0 /root/.kube

COPY --from=builder /go/src/github.com/choerodon/choerodon-cluster-agent/choerodon-cluster-agent /

USER ${USER_UID}

ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["/choerodon-cluster-agent"]