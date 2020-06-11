load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
         "kuberuntime_registry.go",
         "fake_runtime_manager.go"
    ],
    importpath = "k8s.io/kubernetes/pkg/kubelet/runtimeregistry",
    visibility = ["//visibility:public"],
    deps = [
        "//pkg/kubelet/remote:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//staging/src/k8s.io/cri-api/pkg/apis:go_default_library",
        "//staging/src/k8s.io/cri-api/pkg/apis/runtime/v1alpha2:go_default_library",
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
    srcs = [
        ":package-srcs",
        "//pkg/kubelet/runtimeregistry:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = ["kuberuntime_registry_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//pkg/kubelet/remote:go_default_library",
        "//pkg/kubelet/remote/fake:go_default_library",
        "//vendor/github.com/stretchr/testify/assert:go_default_library",
    ],
)
