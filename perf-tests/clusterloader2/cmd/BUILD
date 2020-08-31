load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["clusterloader.go"],
    importpath = "k8s.io/kubernetes/perf-tests/clusterloader2/cmd",
    visibility = ["//visibility:private"],
    deps = [
        "//perf-tests/clusterloader2/api:go_default_library",
        "//perf-tests/clusterloader2/pkg/config:go_default_library",
        "//perf-tests/clusterloader2/pkg/errors:go_default_library",
        "//perf-tests/clusterloader2/pkg/execservice:go_default_library",
        "//perf-tests/clusterloader2/pkg/flags:go_default_library",
        "//perf-tests/clusterloader2/pkg/framework:go_default_library",
        "//perf-tests/clusterloader2/pkg/framework/config:go_default_library",
        "//perf-tests/clusterloader2/pkg/imagepreload:go_default_library",
        "//perf-tests/clusterloader2/pkg/measurement/common:go_default_library",
        "//perf-tests/clusterloader2/pkg/measurement/common/bundle:go_default_library",
        "//perf-tests/clusterloader2/pkg/measurement/common/probes:go_default_library",
        "//perf-tests/clusterloader2/pkg/measurement/common/slos:go_default_library",
        "//perf-tests/clusterloader2/pkg/prometheus:go_default_library",
        "//perf-tests/clusterloader2/pkg/test:go_default_library",
        "//perf-tests/clusterloader2/pkg/util:go_default_library",
        "//pkg/master/ports:go_default_library",
        "//staging/src/k8s.io/api/core/v1:go_default_library",
        "//staging/src/k8s.io/client-go/kubernetes:go_default_library",
        "//vendor/github.com/onsi/ginkgo/config:go_default_library",
        "//vendor/github.com/onsi/ginkgo/reporters:go_default_library",
        "//vendor/github.com/onsi/ginkgo/types:go_default_library",
        "//vendor/k8s.io/klog:go_default_library",
    ],
)

go_binary(
    name = "cmd",
    embed = [":go_default_library"],
    visibility = ["//visibility:public"],
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
