package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

type testToolchain struct {
	workDir         string
	protocGenGoEcho string
	protocGenGo     string
}

var cachedTestToolchain testToolchain

const testAnnotationsProto = `syntax = "proto3";

package google.api;

option go_package = "google.golang.org/genproto/googleapis/api/annotations;annotations";

import "google/api/http.proto";
import "google/protobuf/descriptor.proto";

extend google.protobuf.MethodOptions {
	HttpRule http = 72295728;
}
`

const testHTTPProto = `syntax = "proto3";

package google.api;

option go_package = "google.golang.org/genproto/googleapis/api/annotations;annotations";

message Http {
	repeated HttpRule rules = 1;
	bool fully_decode_reserved_expansion = 2;
}

message HttpRule {
	string selector = 1;
	oneof pattern {
		string get = 2;
		string put = 3;
		string post = 4;
		string delete = 5;
		string patch = 6;
		CustomHttpPattern custom = 8;
	}
	string body = 7;
	string response_body = 12;
	repeated HttpRule additional_bindings = 11;
}

message CustomHttpPattern {
	string kind = 1;
	string path = 2;
}
`

const testUploadProto = `syntax = "proto3";

package test.v1;

option go_package = "example.com/test/v1;testv1";

import "google/protobuf/descriptor.proto";

extend google.protobuf.MethodOptions {
	bool upload = 50007;
}

message UploadRequest {
	string name = 1;
}

message UploadReply {
	string value = 1;
}

service UploadService {
	// @summary 上传文件
	rpc Upload(UploadRequest) returns (UploadReply) {
		option (upload) = true;
	}
}
`

type compileCase struct {
	name         string
	protoContent string
	extraFiles   map[string]string
}

func TestMain(m *testing.M) {
	toolchain, err := prepareTestToolchain()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "准备测试工具链失败: %v\n", err)
		os.Exit(1)
	}
	cachedTestToolchain = toolchain
	code := m.Run()
	if toolchain.workDir != "" {
		_ = os.RemoveAll(toolchain.workDir)
	}
	os.Exit(code)
}

func runGenerate(t *testing.T, opts []string) string {
	t.Helper()
	return runGenerateProto(t, testUploadProto, opts)
}

func runGenerateProto(t *testing.T, protoContent string, opts []string) string {
	t.Helper()
	content, errOutput := runGenerateProtoInternal(t, protoContent, nil, opts)
	if errOutput != "" {
		t.Fatalf("执行 protoc 失败:\n%s", errOutput)
	}
	return content
}

func runGenerateProtoWithExtras(t *testing.T, protoContent string, extraFiles map[string]string, opts []string) string {
	t.Helper()
	content, errOutput := runGenerateProtoInternal(t, protoContent, extraFiles, opts)
	if errOutput != "" {
		t.Fatalf("执行 protoc 失败:\n%s", errOutput)
	}
	return content
}

func runGenerateProtoExpectError(t *testing.T, protoContent string, opts []string) (string, string) {
	t.Helper()
	return runGenerateProtoInternal(t, protoContent, nil, opts)
}

func runGenerateProtoInternal(t *testing.T, protoContent string, extraFiles map[string]string, opts []string) (string, string) {
	t.Helper()

	toolchain := requireTestToolchain(t)
	tmpDir := t.TempDir()
	writeGoogleAPIProtos(t, tmpDir)
	writeExtraProtoFiles(t, tmpDir, extraFiles)

	protoPath := filepath.Join(tmpDir, "test.proto")
	if err := os.WriteFile(protoPath, []byte(protoContent), 0o600); err != nil {
		t.Fatalf("写入测试 proto 失败: %v", err)
	}

	protocArgs := []string{
		"--plugin=protoc-gen-go-echo=" + toolchain.protocGenGoEcho,
		"--go-echo_out=" + tmpDir,
		"--go-echo_opt=" + strings.Join(opts, ","),
		"--proto_path=" + tmpDir,
		"test.proto",
	}

	protocCmd := exec.Command("protoc", protocArgs...)
	protocCmd.Dir = tmpDir
	if output, err := protocCmd.CombinedOutput(); err != nil {
		return "", string(output)
	}

	generatedPath := filepath.Join(tmpDir, "test_echo.pb.go")
	content, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatalf("读取生成文件失败: %v", err)
	}
	return string(content), ""
}

func compileGeneratedPackages(t *testing.T, cases ...compileCase) {
	t.Helper()

	toolchain := requireTestToolchain(t)
	tmpDir := t.TempDir()
	writeGoogleAPIProtos(t, tmpDir)
	for _, testCase := range cases {
		writeExtraProtoFiles(t, tmpDir, testCase.extraFiles)
		caseDir := filepath.Join(tmpDir, testCase.name)
		if err := os.MkdirAll(caseDir, 0o755); err != nil {
			t.Fatalf("创建编译测试目录失败: %v", err)
		}
		protoPath := filepath.Join(caseDir, "test.proto")
		if err := os.WriteFile(protoPath, []byte(testCase.protoContent), 0o600); err != nil {
			t.Fatalf("写入编译测试 proto 失败: %v", err)
		}

		protocArgs := []string{
			"--plugin=protoc-gen-go=" + toolchain.protocGenGo,
			"--plugin=protoc-gen-go-echo=" + toolchain.protocGenGoEcho,
			"--go_out=paths=source_relative:" + tmpDir,
			"--go-echo_out=paths=source_relative:" + tmpDir,
			"--proto_path=" + tmpDir,
		}
		extraInputs := mapKeys(testCase.extraFiles)
		slices.Sort(extraInputs)
		protocArgs = append(protocArgs, extraInputs...)
		protocArgs = append(protocArgs, filepath.ToSlash(filepath.Join(testCase.name, "test.proto")))

		protocCmd := exec.Command("protoc", protocArgs...)
		protocCmd.Dir = tmpDir
		if output, err := protocCmd.CombinedOutput(); err != nil {
			t.Fatalf("执行编译测试 protoc 失败: %v\n%s", err, output)
		}
	}

	goMod := "module example.com/compile\n\ngo 1.26.1\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0o600); err != nil {
		t.Fatalf("写入 go.mod 失败: %v", err)
	}

	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	if output, err := tidyCmd.CombinedOutput(); err != nil {
		t.Fatalf("编译测试依赖整理失败: %v\n%s", err, output)
	}

	buildCmd := exec.Command("go", "build", "./...")
	buildCmd.Dir = tmpDir
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("生成代码应可编译，实际失败: %v\n%s", err, output)
	}
}

func prepareTestToolchain() (testToolchain, error) {
	workDir, err := os.MkdirTemp("", "protoc-gen-go-echo-test-tools-")
	if err != nil {
		return testToolchain{}, fmt.Errorf("创建测试工具目录失败: %w", err)
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return testToolchain{}, fmt.Errorf("获取工作目录失败: %w", err)
	}

	toolchain := testToolchain{
		workDir:         workDir,
		protocGenGoEcho: toolBinaryPath(workDir, "protoc-gen-go-echo"),
		protocGenGo:     toolBinaryPath(workDir, "protoc-gen-go"),
	}

	if err := buildToolBinary(repoRoot, toolchain.protocGenGoEcho, "."); err != nil {
		return testToolchain{}, err
	}
	if err := buildToolBinary(repoRoot, toolchain.protocGenGo, "google.golang.org/protobuf/cmd/protoc-gen-go"); err != nil {
		return testToolchain{}, err
	}
	return toolchain, nil
}

func requireTestToolchain(t *testing.T) testToolchain {
	t.Helper()
	if cachedTestToolchain.protocGenGoEcho == "" || cachedTestToolchain.protocGenGo == "" {
		t.Fatal("测试工具链未初始化")
	}
	return cachedTestToolchain
}

func toolBinaryPath(root, binaryName string) string {
	binaryPath := filepath.Join(root, binaryName)
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}
	return binaryPath
}

func buildToolBinary(workDir, outputPath, target string) error {
	buildCmd := exec.Command("go", "build", "-o", outputPath, target)
	buildCmd.Dir = workDir
	if output, err := buildCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("构建 %s 失败: %w\n%s", filepath.Base(outputPath), err, output)
	}
	return nil
}

func mapKeys(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

func writeGoogleAPIProtos(t *testing.T, root string) {
	t.Helper()
	annotationsPath := filepath.Join(root, "google", "api", "annotations.proto")
	httpPath := filepath.Join(root, "google", "api", "http.proto")
	if err := os.MkdirAll(filepath.Dir(annotationsPath), 0o755); err != nil {
		t.Fatalf("创建 google/api 目录失败: %v", err)
	}
	if err := os.WriteFile(annotationsPath, []byte(testAnnotationsProto), 0o600); err != nil {
		t.Fatalf("写入 annotations.proto 失败: %v", err)
	}
	if err := os.WriteFile(httpPath, []byte(testHTTPProto), 0o600); err != nil {
		t.Fatalf("写入 http.proto 失败: %v", err)
	}
}

func writeExtraProtoFiles(t *testing.T, root string, extraFiles map[string]string) {
	t.Helper()
	for relativePath, content := range extraFiles {
		fullPath := filepath.Join(root, relativePath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("创建额外 proto 目录失败: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o600); err != nil {
			t.Fatalf("写入额外 proto 文件失败: %v", err)
		}
	}
}
