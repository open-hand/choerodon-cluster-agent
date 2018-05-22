# Choerodon Agent

[中文](README_CN.md)

The environment client connects to the choerodon platform through websocket. The two parties interact through `command/response` to complete the functions: management of helm release, network management, k8s object monitoring, and container log and shell.

![](image/design.png)

## Feature

- [x] helm release management
- [x] Web Services and Domain Management
- [x] K8s object monitoring and processing
- [x] Container log and shell

## Requirements

- Go 1.9.4 and above
- [Dep](https://github.com/golang/dep)

## Installation and Getting Started

Build

```bash
make
```

Run

```bash
./bin/choerodon-agent \
    --v=1 \
    --tiller-connection-timeout=2 \
    --connect=[Server address] \
    --token=[Token] \
    --namespace=[k8s namespace]
```

## Reporting Issues

If you find any shortcomings or bugs, please describe them in the Issue.
    
## How to Contribute

Pull requests are welcome! Follow this link for more information on how to contribute.
