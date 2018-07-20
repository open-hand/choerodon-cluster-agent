## Choerodon Agent

```yaml
config:
  # The WebSocket Address of devops-service
  connect:
  #the token of this environment
  token: ""
  tillerConnectionTimeout: 2
  port: 8088
  logLevel: 0
  #the environment Id
  envId: ""

```

## Resource
`Role`:
  In the RBAC API, a role contains rules that represent a set of permissions. Permissions are purely additive (there are no “deny” rules).

`RoleBinding`:
  A RoleBinding may reference a Role in the same namespace. The following RoleBinding grants the “pod-reader” role to the user “jane” within the “default” namespace. This allows “jane” to read pods in the “default” namespace.

`ServiceAccount`:
The ServiceAccount is access the API server, and when you create a pod, if you do not specify a service account, it is automatically assigned the default service account in the same namespace 

