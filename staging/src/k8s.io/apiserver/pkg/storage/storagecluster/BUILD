load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "storageclustermanager.go",
        "tenantstoragemapmanager.go",
        "update.go",
    ],
    importmap = "k8s.io/kubernetes/vendor/k8s.io/apiserver/pkg/storage/storagecluster",
    importpath = "k8s.io/apiserver/pkg/storage/storagecluster",
    visibility = ["//visibility:public"],
    deps = [
        "//staging/src/k8s.io/api/core/v1:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/labels:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/util/diff:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/util/errors:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/util/runtime:go_default_library",
        "//staging/src/k8s.io/client-go/informers/core/v1:go_default_library",
        "//staging/src/k8s.io/client-go/listers/core/v1:go_default_library",
        "//staging/src/k8s.io/client-go/tools/cache:go_default_library",
        "//vendor/github.com/grafov/bcast:go_default_library",
        "//vendor/k8s.io/klog:go_default_library",
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

go_test(
    name = "go_default_test",
    srcs = [
        "storageclustermanager_test.go",
        "tenantstoragemapmanager_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//staging/src/k8s.io/api/core/v1:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//staging/src/k8s.io/client-go/informers:go_default_library",
        "//staging/src/k8s.io/client-go/kubernetes/fake:go_default_library",
        "//vendor/github.com/stretchr/testify/assert:go_default_library",
    ],
)
