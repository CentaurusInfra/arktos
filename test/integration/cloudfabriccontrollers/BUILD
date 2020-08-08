load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "deployment.go",
        "deployment_util.go",
        "replicaset.go",
        "testutil.go",
    ],
    importpath = "k8s.io/kubernetes/test/integration/cloudfabriccontrollers",
    visibility = ["//visibility:public"],
    deps = [
        "//pkg/api/v1/pod:go_default_library",
        "//pkg/cloudfabric-controller/controllerframework:go_default_library",
        "//pkg/cloudfabric-controller/deployment:go_default_library",
        "//pkg/cloudfabric-controller/deployment/util:go_default_library",
        "//pkg/cloudfabric-controller/replicaset:go_default_library",
        "//pkg/util/labels:go_default_library",
        "//staging/src/k8s.io/api/apps/v1:go_default_library",
        "//staging/src/k8s.io/api/core/v1:go_default_library",
        "//staging/src/k8s.io/api/extensions/v1beta1:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/apis/meta/fuzzer:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/types:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/util/intstr:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/util/uuid:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/util/wait:go_default_library",
        "//staging/src/k8s.io/client-go/informers:go_default_library",
        "//staging/src/k8s.io/client-go/kubernetes:go_default_library",
        "//staging/src/k8s.io/client-go/rest:go_default_library",
        "//staging/src/k8s.io/client-go/tools/cache:go_default_library",
        "//test/integration/framework:go_default_library",
        "//vendor/github.com/davecgh/go-spew/spew:go_default_library",
        "//vendor/github.com/grafov/bcast:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    size = "large",
    srcs = [
        "deployment_test.go",
        "framework_test.go",
        "main_test.go",
        "replicaset_test.go",
    ],
    embed = [":go_default_library"],
    tags = ["integration"],
    deps = [
        "//staging/src/k8s.io/api/core/v1:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//test/integration/framework:go_default_library",
        "//vendor/github.com/stretchr/testify/assert:go_default_library",
    ],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [":package-srcs"],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
