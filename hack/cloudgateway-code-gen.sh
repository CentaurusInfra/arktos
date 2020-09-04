#!/bin/bash

set -x

ROOT_PACKAGE="k8s.io/kubernetes"
CUSTOM_RESOURCE_NAME="cloudgateway"
CUSTOM_RESOURCE_VERSION="v1"
GO111MODULE=off
CODE_GENERATOR="$GOPATH/src/github.com/arktos/vendor/k8s.io/code-generator"

# execute code gen, pkg/client is the client dir, pkg/apis is type define dir
cd $CODE_GENERATOR && ./generate-groups.sh all "$ROOT_PACKAGE/pkg/client" "$ROOT_PACKAGE/pkg/apis" "$CUSTOM_RESOURCE_NAME:$CUSTOM_RESOURCE_VERSION"