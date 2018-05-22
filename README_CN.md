# Choerodon Agent

[English](README.md)

环境客户端，通过websocket方式连接到猪齿鱼平台，双方通过`command/response`方式来进行交互，来完成helm release的管理、网络管理、k8s对象监听和容器日志和shell等功能。

![](image/design.png)

## Feature

- [x] helm release管理
- [x] 网络服务和域名管理
- [x] k8s对象监听及处理
- [x] 容器日志及shell

## Requirements

- Go 1.9.4及以上
- [Dep](https://github.com/golang/dep)

## Installation and Getting Started

构建
```bash
make
```

运行
```bash
./bin/choerodon-agent \
    --v=1 \
    --tiller-connection-timeout=2 \
    --connect=[服务端地址] \
    --token=[Token] \
    --namespace=[k8s namespace]
```

## Reporting Issues

If you find any shortcomings or bugs, please describe them in the Issue.
    
## How to Contribute
Pull requests are welcome! Follow this link for more information on how to contribute.
