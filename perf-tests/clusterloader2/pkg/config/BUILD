load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "cluster.go",
        "codec.go",
        "template_functions.go",
        "template_provider.go",
    ],
    importpath = "k8s.io/kubernetes/perf-tests/clusterloader2/pkg/config",
    visibility = ["//visibility:public"],
    deps = [
        "//perf-tests/clusterloader2/api:go_default_library",
        "//perf-tests/clusterloader2/pkg/errors:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/apis/meta/v1/unstructured:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/util/yaml:go_default_library",
        "//staging/src/k8s.io/client-go/kubernetes/scheme:go_default_library",
        "//vendor/gopkg.in/yaml.v2:go_default_library",
        "//vendor/k8s.io/klog:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["template_provider_test.go"],
    embed = [":go_default_library"],
    deps = ["//perf-tests/clusterloader2/api:go_default_library"],
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
