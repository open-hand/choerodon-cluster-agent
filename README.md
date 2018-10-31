# Choerodon Agent

`Choerodon Agent` is a environment client which connects to the choerodon platform through websocket, And it is a relay station for other services and k8s interaction. The interactioninteract through `command/response` to provide these features to other service,such as management of helm release, network management, k8s object monitoring, and container log and shell. We can use che choerodon agent to operate the k8s like using kubectl client.

![](image/design.png)

## Feature

- [x] helm release management
- [x] Web Services and Domain Management
- [x] K8s object monitoring and processing
- [x] Container log and shell
- [x] WebSocket log of k8s object

## Requirements

- Go 1.9.4 and above
- [Dep](https://github.com/golang/dep)

## Installation and Run

Build

```bash
make
```

Run

```bash
./bin/choerodon-cluster-agent \
    --v=1 \
    --tiller-connection-timeout=2 \
    --connect=[Server address] \
    --token=[Token] \
    --namespace=[k8s namespace]
```

## Reporting Issues
If you find any shortcomings or bugs, please describe them in the [issue](https://github.com/choerodon/choerodon/issues/new?template=issue_template.md).

## How to Contribute
Pull requests are welcome! [Follow](https://github.com/choerodon/choerodon/blob/master/CONTRIBUTING.md) to know for more information on how to contribute.
