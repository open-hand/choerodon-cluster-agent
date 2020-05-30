FROM dockerhub.k8s.saas.hand-china.com/library/golang:1.13 as builder
WORKDIR /go/src/github.com/choerodon/choerodon-cluster-agent
COPY . .
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GO111MODULE=on go build -o ./choerodon-cluster-agent -mod=vendor -v ./cmd/manager

FROM dockerhub.k8s.saas.hand-china.com/library/debian:stretch

ENV USER_UID=1001 \
    USER_NAME=choerodon-cluster-agent \
    TINI_VERSION=v0.19.0

RUN cp /etc/apt/sources.list /etc/apt/sources.list.bak && \
    sed -i 's http://.*.debian.org http://mirrors.aliyun.com g' /etc/apt/sources.list && \
    apt-get update && \
    apt-get install -y \
        git \
        curl \
        tzdata \
        ca-certificates && \
    mkdir -p /ssh-keys && \
    curl -sSL -o /usr/bin/kubectl \
       "http://mirror.azure.cn/kubernetes/kubectl/$(curl -sSL http://mirror.azure.cn/kubernetes/kubectl/stable.txt)/bin/linux/amd64/kubectl" && \
    chmod a+x /usr/bin/kubectl && \
    curl -sSL -o /usr/bin/tini \
        "https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini" && \
    chmod a+x /usr/bin/tini && \
    ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

RUN echo "${USER_NAME}:x:${USER_UID}:0:${USER_NAME} user:${HOME}:/sbin/nologin" >> /etc/passwd \
    && mkdir -p ${HOME} \
    && mkdir -p /ssh-keys \
    && mkdir -p /polaris \
    && mkdir -p /tmp \
    && mkdir -p /etc/ssh \
    && chown ${USER_UID}:0 ${HOME} \
    && chown ${USER_UID}:0 /ssh-keys \
    && chown ${USER_UID}:0 /polaris \
    && chown ${USER_UID}:0 /tmp \
    && chown ${USER_UID}:0 /etc/ssh/ssh_config

COPY --from=builder /go/src/github.com/choerodon/choerodon-cluster-agent/choerodon-cluster-agent /

USER ${USER_UID}

ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["/choerodon-cluster-agent"]