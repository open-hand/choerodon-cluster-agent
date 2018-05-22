FROM registry.cn-hangzhou.aliyuncs.com/choerodon-tools/golang:1.9.4-alpine3.7 as builder
WORKDIR /go/src/github.com/choerodon/choerodon-agent
COPY . .
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build .

FROM registry.cn-hangzhou.aliyuncs.com/choerodon-tools/alpine:c7n
WORKDIR /
COPY --from=builder /go/src/github.com/choerodon/choerodon-agent/choerodon-agent .
CMD ["/choerodon-agent"]
