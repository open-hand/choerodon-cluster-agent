#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

vendor/k8s.io/code-generator/generate-groups.sh \
  "deepcopy,client,informer,lister" \
  github.com/choerodon/choerodon-cluster-agent/pkg/client \
  github.com/choerodon/choerodon-cluster-agent/pkg/apis \
  choerodon:v1alpha1 \
  --go-header-file "./scripts/boilerplate.go.txt"
