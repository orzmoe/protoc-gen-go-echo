package main

import (
	"flag"
	"fmt"
	"runtime/debug"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

// version 优先使用 ldflags 注入值，否则从 Go 模块元信息读取（go install 场景）。
var version = ""

func resolveVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "(devel)"
}

const (
	responseStyleWrapped = "wrapped"
	responseStyleDirect  = "direct"
)

type generatorConfig struct {
	responseStyle string
	packageNames  map[protogen.GoImportPath]protogen.GoPackageName
}

func newGeneratorConfig(gen *protogen.Plugin, responseStyle string) generatorConfig {
	packageNames := make(map[protogen.GoImportPath]protogen.GoPackageName)
	for _, file := range gen.Files {
		registerGoPackageName(packageNames, file)
	}
	registerWellKnownPackage(packageNames, "google.golang.org/protobuf/types/known/emptypb", "emptypb")
	return generatorConfig{
		responseStyle: responseStyle,
		packageNames:  packageNames,
	}
}

func registerGoPackageName(packageNames map[protogen.GoImportPath]protogen.GoPackageName, file *protogen.File) {
	if file == nil {
		return
	}
	if file.GoImportPath != "" && file.GoPackageName != "" {
		packageNames[file.GoImportPath] = file.GoPackageName
	}
}

func registerWellKnownPackage(packageNames map[protogen.GoImportPath]protogen.GoPackageName, importPath protogen.GoImportPath, packageName protogen.GoPackageName) {
	if importPath == "" || packageName == "" {
		return
	}
	packageNames[importPath] = packageName
}

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("protoc-gen-go-echo %v\n", resolveVersion()) //nolint:forbidigo // 命令行版本输出
		return
	}

	var flags flag.FlagSet
	responseStyle := flags.String("response_style", responseStyleWrapped, "response style: wrapped or direct")

	options := protogen.Options{
		ParamFunc: flags.Set,
	}
	options.Run(func(gen *protogen.Plugin) error {
		var config generatorConfig
		switch *responseStyle {
		case responseStyleWrapped, responseStyleDirect:
			config = newGeneratorConfig(gen, *responseStyle)
		default:
			return fmt.Errorf("invalid response_style=%q, expected wrapped|direct", *responseStyle)
		}

		gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			generateFile(gen, f, config)
		}
		return nil
	})
}
