FROM golang:1.13 as builder
WORKDIR /go/src/github.com/choerodon/choerodon-cluster-agent
COPY . .
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GO111MODULE=on go build -o ./choerodon-cluster-agent -mod=vendor -v ./cmd/manager

FROM debian:stretch

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
    curl -sSLO /usr/bin/kubectl \
       "http://mirror.azure.cn/kubernetes/kubectl/$(curl -sSL http://mirror.azure.cn/kubernetes/kubectl/stable.txt)/bin/linux/amd64/kubectl" && \
    chmod a+x /usr/bin/kubectl && \
    curl -sSLo /usr/bin/tini \
        "https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini" && \
    chmod a+x /usr/bin/tini && \
    ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

RUN echo "${USER_NAME}:x:${USER_UID}:0:${USER_NAME} user:${HOME}:/sbin/nologin" >> /etc/passwd \
    && mkdir -p ${HOME} \
    && chown ${USER_UID}:0 ${HOME} \
    && chmod ug+rwx ${HOME}

COPY --chown=${USER_UID}:0 ./build/ssh_config ssh_config
COPY --from=builder /go/src/github.com/choerodon/choerodon-cluster-agent/choerodon-cluster-agent /

ENTRYPOINT ["/sbin/tini", "--"]
CMD ["/choerodon-cluster-agent"]