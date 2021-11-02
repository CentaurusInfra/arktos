load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["openstackutils.go"],
    importmap = "k8s.io/kubernetes/vendor/k8s.io/apiserver/pkg/util/openstack",
    importpath = "k8s.io/apiserver/pkg/util/openstack",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/klog:go_default_library"],
)

exports_files(
    srcs = [
        "openstackRequestTemplate.json",
    ],
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
