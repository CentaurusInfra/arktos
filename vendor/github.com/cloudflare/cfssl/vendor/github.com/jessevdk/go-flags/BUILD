load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "arg.go",
        "closest.go",
        "command.go",
        "completion.go",
        "convert.go",
        "error.go",
        "flags.go",
        "group.go",
        "help.go",
        "ini.go",
        "man.go",
        "multitag.go",
        "option.go",
        "optstyle_other.go",
        "optstyle_windows.go",
        "parser.go",
        "termsize.go",
        "termsize_nosysioctl.go",
        "tiocgwinsz_bsdish.go",
        "tiocgwinsz_linux.go",
        "tiocgwinsz_other.go",
    ],
    importmap = "github.com/cloudflare/cfssl/vendor/github.com/jessevdk/go-flags",
    importpath = "github.com/jessevdk/go-flags",
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
