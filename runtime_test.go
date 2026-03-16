package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const runtimeTestProto = `syntax = "proto3";

package test.runtime.v1;

option go_package = "example.com/runtime;runtime";

import "google/api/annotations.proto";
import "google/protobuf/descriptor.proto";

extend google.protobuf.MethodOptions {
	bool set_auth_cookie = 50001;
	int32 response_mode = 50008;
}

message PingReq { string id = 1; }
message PingReply { string value = 1; }

message DownloadReq { string id = 1; }
message DownloadReply {
	bytes content = 1;
	string filename = 2;
	string content_type = 3;
}

message RedirectReq { string token = 1; }
message RedirectReply { string redirect_url = 1; }

message LoginReq { string username = 1; string password = 2; }
message LoginReply { string access_token = 1; int64 expires_in = 2; }

service RuntimeService {
	// @summary 健康检查
	rpc Ping(PingReq) returns (PingReply);

	// @summary 获取详情
	rpc GetItem(PingReq) returns (PingReply) {
		option (google.api.http) = {
			get: "/items/{id}"
		};
	}

	// @summary 下载文件
	rpc Download(DownloadReq) returns (DownloadReply) {
		option (response_mode) = 2;
	}

	// @summary 重定向
	rpc Redirect(RedirectReq) returns (RedirectReply) {
		option (response_mode) = 3;
	}

	// @summary 登录
	rpc Login(LoginReq) returns (LoginReply) {
		option (set_auth_cookie) = true;
	}
}
`

const runtimeTestFile = `package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// --- fake 实现 ---

type fakeServer struct {
	pingReply       *PingReply
	pingErr         error
	downloadReply   *DownloadReply
	downloadErr     error
	redirectReply   *RedirectReply
	redirectErr     error
	loginReply      *LoginReply
	loginErr        error
}

func (f *fakeServer) Ping(_ context.Context, _ *PingReq) (*PingReply, error) {
	return f.pingReply, f.pingErr
}
func (f *fakeServer) GetItem(_ context.Context, _ *PingReq) (*PingReply, error) {
	return f.pingReply, f.pingErr
}
func (f *fakeServer) Download(_ context.Context, _ *DownloadReq) (*DownloadReply, error) {
	return f.downloadReply, f.downloadErr
}
func (f *fakeServer) Redirect(_ context.Context, _ *RedirectReq) (*RedirectReply, error) {
	return f.redirectReply, f.redirectErr
}
func (f *fakeServer) Login(_ context.Context, _ *LoginReq) (*LoginReply, error) {
	return f.loginReply, f.loginErr
}

type allowAllPerm struct{}

func (allowAllPerm) CheckPermission(_ echo.Context, _ string) error { return nil }
func (allowAllPerm) CheckAllPermissions(_ echo.Context, _ []string) error { return nil }
func (allowAllPerm) CheckAnyPermission(_ echo.Context, _ []string) error { return nil }
func (allowAllPerm) CheckAuth(_ echo.Context) error { return nil }

type validatorStub struct {
	err error
}

func (v validatorStub) Validate(any) error { return v.err }

func setRuntimeFlags(t *testing.T, isDev, detailed bool) {
	t.Helper()
	oldDev := runtimeServiceIsDev
	oldDetailed := runtimeServiceIsDetailedValidation
	runtimeServiceIsDev = isDev
	runtimeServiceIsDetailedValidation = detailed
	t.Cleanup(func() {
		runtimeServiceIsDev = oldDev
		runtimeServiceIsDetailedValidation = oldDetailed
	})
}

func setupEcho(srv RuntimeServiceHTTPServer) *echo.Echo {
	e := echo.New()
	g := e.Group("")
	RegisterRuntimeServiceHTTPServer(g, srv, allowAllPerm{})
	return e
}

// --- 测试 ---

func TestRuntime_PingWithoutValidator(t *testing.T) {
	srv := &fakeServer{pingReply: &PingReply{Value: "pong"}}
	e := setupEcho(srv)

	req := httptest.NewRequest(http.MethodPost, "/ping", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("未注册 Validator 时应返回 200，实际: %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestRuntime_GRPCNotFoundMapsTo404(t *testing.T) {
	srv := &fakeServer{pingErr: status.Error(codes.NotFound, "not found")}
	e := setupEcho(srv)

	req := httptest.NewRequest(http.MethodPost, "/ping", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != 404 {
		t.Fatalf("gRPC NotFound 应映射到 HTTP 404，实际: %d", rec.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应 JSON 失败: %v", err)
	}
	if msg, _ := body["msg"].(string); msg != "not found" {
		t.Fatalf("错误消息应为 'not found'，实际: %q", msg)
	}
}

func TestRuntime_GRPCPermissionDeniedMapsTo403(t *testing.T) {
	srv := &fakeServer{pingErr: status.Error(codes.PermissionDenied, "forbidden")}
	e := setupEcho(srv)

	req := httptest.NewRequest(http.MethodPost, "/ping", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != 403 {
		t.Fatalf("gRPC PermissionDenied 应映射到 HTTP 403，实际: %d", rec.Code)
	}
}

func TestRuntime_ErrorDetailHiddenOutsideDevelopment(t *testing.T) {
	setRuntimeFlags(t, false, false)
	srv := &fakeServer{pingErr: errors.New("database unavailable")}
	e := setupEcho(srv)

	req := httptest.NewRequest(http.MethodPost, "/ping", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != 500 {
		t.Fatalf("普通错误应返回 500，实际: %d, body: %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应 JSON 失败: %v", err)
	}
	for _, key := range []string{"error_detail", "stack_trace", "path", "method"} {
		if _, ok := body[key]; ok {
			t.Fatalf("非开发模式不应返回 %s，实际 body: %v", key, body)
		}
	}
}

func TestRuntime_ErrorDetailVisibleInDevelopment(t *testing.T) {
	setRuntimeFlags(t, true, false)
	srv := &fakeServer{pingErr: errors.New("database unavailable")}
	e := setupEcho(srv)

	req := httptest.NewRequest(http.MethodPost, "/ping", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != 500 {
		t.Fatalf("开发模式普通错误应返回 500，实际: %d, body: %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应 JSON 失败: %v", err)
	}
	if detail, _ := body["error_detail"].(string); detail != "database unavailable" {
		t.Fatalf("开发模式应返回 error_detail，实际: %v", body["error_detail"])
	}
	if stack, _ := body["stack_trace"].(string); stack == "" {
		t.Fatalf("开发模式应返回 stack_trace，实际 body: %v", body)
	}
	if path, _ := body["path"].(string); path != "/ping" {
		t.Fatalf("开发模式应返回 path=/ping，实际: %v", body["path"])
	}
	if method, _ := body["method"].(string); method != http.MethodPost {
		t.Fatalf("开发模式应返回 method=POST，实际: %v", body["method"])
	}
}

func TestRuntime_DetailedValidationDoesNotExposePathOrMethod(t *testing.T) {
	setRuntimeFlags(t, false, true)
	srv := &fakeServer{pingReply: &PingReply{Value: "pong"}}
	e := setupEcho(srv)
	e.Validator = validatorStub{err: mockValidationErrors{{field: "Name", tag: "required"}}}

	req := httptest.NewRequest(http.MethodPost, "/ping", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != 400 {
		t.Fatalf("详细校验模式应返回 400，实际: %d, body: %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应 JSON 失败: %v", err)
	}
	if detail, _ := body["error_detail"].(string); detail == "" {
		t.Fatalf("详细校验模式应返回 error_detail，实际 body: %v", body)
	}
	if _, ok := body["validation_errors"]; !ok {
		t.Fatalf("详细校验模式应返回 validation_errors，实际 body: %v", body)
	}
	for _, key := range []string{"path", "method"} {
		if _, ok := body[key]; ok {
			t.Fatalf("详细校验模式不应返回 %s，实际 body: %v", key, body)
		}
	}
}

func TestRuntime_GRPCUnauthenticatedMapsTo401(t *testing.T) {
	srv := &fakeServer{pingErr: status.Error(codes.Unauthenticated, "unauthenticated")}
	e := setupEcho(srv)

	req := httptest.NewRequest(http.MethodPost, "/ping", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != 401 {
		t.Fatalf("gRPC Unauthenticated 应映射到 HTTP 401，实际: %d", rec.Code)
	}
}

func TestRuntime_FileDownloadResponse(t *testing.T) {
	srv := &fakeServer{downloadReply: &DownloadReply{
		Content:     []byte("hello world"),
		Filename:    "test.txt",
		ContentType: "text/plain",
	}}
	e := setupEcho(srv)

	req := httptest.NewRequest(http.MethodPost, "/download", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("文件下载应返回 200，实际: %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "text/plain" {
		t.Fatalf("Content-Type 应为 text/plain，实际: %q", ct)
	}
	cd := rec.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "test.txt") {
		t.Fatalf("Content-Disposition 应包含文件名，实际: %q", cd)
	}
	body, _ := io.ReadAll(rec.Body)
	if string(body) != "hello world" {
		t.Fatalf("响应体应为文件内容，实际: %q", string(body))
	}
}

func TestRuntime_RedirectResponse(t *testing.T) {
	srv := &fakeServer{redirectReply: &RedirectReply{RedirectUrl: "https://example.com"}}
	e := setupEcho(srv)

	req := httptest.NewRequest(http.MethodPost, "/redirect", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != 302 {
		t.Fatalf("重定向应返回 302，实际: %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if loc != "https://example.com" {
		t.Fatalf("Location 应为 https://example.com，实际: %q", loc)
	}
}

func TestRuntime_AuthCookieResponse(t *testing.T) {
	srv := &fakeServer{loginReply: &LoginReply{
		AccessToken: "my-token-123",
		ExpiresIn:   3600,
	}}
	e := setupEcho(srv)

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("{\"username\":\"u\",\"password\":\"p\"}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("登录应返回 200，实际: %d, body: %s", rec.Code, rec.Body.String())
	}

	cookies := rec.Result().Cookies()
	var authCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "access_token" {
			authCookie = c
			break
		}
	}
	if authCookie == nil {
		t.Fatal("响应应包含 access_token cookie")
	}
	if authCookie.Value != "my-token-123" {
		t.Fatalf("cookie 值应为 my-token-123，实际: %q", authCookie.Value)
	}
	if authCookie.MaxAge != 3600 {
		t.Fatalf("cookie MaxAge 应为 3600，实际: %d", authCookie.MaxAge)
	}
	if !authCookie.HttpOnly {
		t.Fatal("cookie 应设置 HttpOnly")
	}
}

func TestRuntime_WrappedSuccessFormat(t *testing.T) {
	srv := &fakeServer{pingReply: &PingReply{Value: "ok"}}
	e := setupEcho(srv)

	req := httptest.NewRequest(http.MethodPost, "/ping", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应 JSON 失败: %v", err)
	}
	if code, _ := body["code"].(float64); code != 200 {
		t.Fatalf("wrapped 格式 code 应为 200，实际: %v", body["code"])
	}
	if _, ok := body["data"]; !ok {
		t.Fatal("wrapped 格式应包含 data 字段")
	}
}

func TestRuntime_GetItemPathParam(t *testing.T) {
	srv := &fakeServer{pingReply: &PingReply{Value: "found"}}
	e := setupEcho(srv)

	req := httptest.NewRequest(http.MethodGet, "/items/abc123", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("GET /items/:id 应返回 200，实际: %d, body: %s", rec.Code, rec.Body.String())
	}
}

// --- 异常路径测试 ---

func TestRuntime_NilResponseReturnsError(t *testing.T) {
	// 业务层返回 (nil, nil)，应返回受控错误而非 panic
	srv := &fakeServer{} // pingReply 默认为 nil
	e := setupEcho(srv)

	req := httptest.NewRequest(http.MethodPost, "/ping", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != 500 {
		t.Fatalf("nil 响应应返回 500，实际: %d, body: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应 JSON 失败: %v", err)
	}
	if msg, _ := body["msg"].(string); msg == "" {
		t.Fatal("nil 响应的错误消息不应为空")
	}
}

func TestRuntime_FileDownloadNilResponseReturnsError(t *testing.T) {
	// 文件下载 (nil, nil) 不应 panic
	srv := &fakeServer{} // downloadReply 默认为 nil
	e := setupEcho(srv)

	req := httptest.NewRequest(http.MethodPost, "/download", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != 500 {
		t.Fatalf("文件下载 nil 响应应返回 500，实际: %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestRuntime_FileDownloadSpecialFilename(t *testing.T) {
	// 文件名包含引号和特殊字符，Content-Disposition 应安全处理
	srv := &fakeServer{downloadReply: &DownloadReply{
		Content:     []byte("data"),
		Filename:    "test\"file.txt",
		ContentType: "text/plain",
	}}
	e := setupEcho(srv)

	req := httptest.NewRequest(http.MethodPost, "/download", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("特殊文件名下载应返回 200，实际: %d", rec.Code)
	}
	cd := rec.Header().Get("Content-Disposition")
	if cd == "" {
		t.Fatal("Content-Disposition 不应为空")
	}
	// 不应包含未转义的引号
	if strings.Contains(cd, "test\"file") {
		t.Fatalf("Content-Disposition 应正确转义文件名中的引号，实际: %q", cd)
	}
}

func TestRuntime_RedirectNilResponseReturnsError(t *testing.T) {
	// 重定向 (nil, nil) 不应 panic
	srv := &fakeServer{} // redirectReply 默认为 nil
	e := setupEcho(srv)

	req := httptest.NewRequest(http.MethodPost, "/redirect", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != 500 {
		t.Fatalf("重定向 nil 响应应返回 500，实际: %d, body: %s", rec.Code, rec.Body.String())
	}
}

// --- parseValidationErrors 测试 ---

// mockFieldError 模拟实现 fieldError 接口的单个字段错误
type mockFieldError struct {
	field string
	tag   string
	param string
}

func (e mockFieldError) Field() string { return e.field }
func (e mockFieldError) Tag() string   { return e.tag }
func (e mockFieldError) Param() string { return e.param }
func (e mockFieldError) Error() string { return e.field + ": " + e.tag }

// mockValidationErrors 模拟 go-playground/validator/v10.ValidationErrors 的底层类型：
// 它是一个 []FieldError，同时实现了 error 接口，但不实现 Unwrap() []error。
type mockValidationErrors []mockFieldError

func (ve mockValidationErrors) Error() string {
	msgs := make([]string, len(ve))
	for i, e := range ve {
		msgs[i] = e.Error()
	}
	return strings.Join(msgs, "; ")
}

// mockUnwrapErrors 模拟实现 Go 1.20+ 的 Unwrap() []error 接口
type mockUnwrapErrors struct {
	errs []error
}

func (e mockUnwrapErrors) Error() string { return "multiple errors" }
func (e mockUnwrapErrors) Unwrap() []error { return e.errs }

func TestRuntime_ParseValidationErrors_MultiFieldSlice(t *testing.T) {
	// 模拟 validator.ValidationErrors（底层为 slice，元素实现 fieldError）
	verr := mockValidationErrors{
		{field: "Name", tag: "required", param: ""},
		{field: "Email", tag: "email", param: ""},
		{field: "Age", tag: "gte", param: "18"},
	}
	result := parseRuntimeServiceValidationErrors(verr)
	if len(result) != 3 {
		t.Fatalf("应提取 3 个字段错误，实际: %d, result: %v", len(result), result)
	}
	if result[0].Field != "Name" || result[0].Tag != "required" {
		t.Fatalf("第 1 个字段错误不匹配，实际: %v", result[0])
	}
	if result[1].Field != "Email" || result[1].Tag != "email" {
		t.Fatalf("第 2 个字段错误不匹配，实际: %v", result[1])
	}
	if result[2].Field != "Age" || result[2].Tag != "gte" || result[2].Param != "18" {
		t.Fatalf("第 3 个字段错误不匹配，实际: %v", result[2])
	}
}

func TestRuntime_ParseValidationErrors_SingleFieldError(t *testing.T) {
	// 单个 fieldError（非 slice），应走 errors.As 兜底
	verr := mockFieldError{field: "Token", tag: "required", param: ""}
	result := parseRuntimeServiceValidationErrors(verr)
	if len(result) != 1 {
		t.Fatalf("应提取 1 个字段错误，实际: %d, result: %v", len(result), result)
	}
	if result[0].Field != "Token" || result[0].Tag != "required" {
		t.Fatalf("字段错误不匹配，实际: %v", result[0])
	}
}

func TestRuntime_ParseValidationErrors_UnwrapMultipleErrors(t *testing.T) {
	// Go 1.20+ Unwrap() []error 路径
	verr := mockUnwrapErrors{
		errs: []error{
			mockFieldError{field: "A", tag: "min", param: "1"},
			mockFieldError{field: "B", tag: "max", param: "100"},
		},
	}
	result := parseRuntimeServiceValidationErrors(verr)
	if len(result) != 2 {
		t.Fatalf("应提取 2 个字段错误，实际: %d, result: %v", len(result), result)
	}
	if result[0].Field != "A" || result[1].Field != "B" {
		t.Fatalf("字段错误不匹配，实际: %v", result)
	}
}

func TestRuntime_ParseValidationErrors_NilError(t *testing.T) {
	// nil error 不应 panic
	result := parseRuntimeServiceValidationErrors(nil)
	if len(result) != 0 {
		t.Fatalf("nil error 应返回空切片，实际: %v", result)
	}
}

func TestRuntime_ParseValidationErrors_PlainError(t *testing.T) {
	// 普通 error（不实现 fieldError），应返回空切片
	verr := errors.New("some random error")
	result := parseRuntimeServiceValidationErrors(verr)
	if len(result) != 0 {
		t.Fatalf("普通 error 应返回空切片，实际: %v", result)
	}
}
`

func TestGenerate_RuntimeBehavior(t *testing.T) {
	toolchain := requireTestToolchain(t)
	tmpDir := t.TempDir()
	writeGoogleAPIProtos(t, tmpDir)

	// 写入 proto
	caseDir := filepath.Join(tmpDir, "runtime")
	if err := os.MkdirAll(caseDir, 0o755); err != nil {
		t.Fatalf("创建运行时测试目录失败: %v", err)
	}
	protoPath := filepath.Join(caseDir, "test.proto")
	if err := os.WriteFile(protoPath, []byte(runtimeTestProto), 0o600); err != nil {
		t.Fatalf("写入运行时测试 proto 失败: %v", err)
	}

	// 生成 Go 代码
	protocArgs := []string{
		"--plugin=protoc-gen-go=" + toolchain.protocGenGo,
		"--plugin=protoc-gen-go-echo=" + toolchain.protocGenGoEcho,
		"--go_out=paths=source_relative:" + tmpDir,
		"--go-echo_out=paths=source_relative:" + tmpDir,
		"--proto_path=" + tmpDir,
		"runtime/test.proto",
	}
	protocCmd := exec.Command("protoc", protocArgs...)
	protocCmd.Dir = tmpDir
	if output, err := protocCmd.CombinedOutput(); err != nil {
		t.Fatalf("运行时测试 protoc 失败: %v\n%s", err, output)
	}

	// 写入运行时测试文件
	testPath := filepath.Join(caseDir, "runtime_test.go")
	if err := os.WriteFile(testPath, []byte(runtimeTestFile), 0o600); err != nil {
		t.Fatalf("写入运行时测试文件失败: %v", err)
	}

	// 初始化 go module
	goMod := "module example.com/runtime\n\ngo 1.26.1\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0o600); err != nil {
		t.Fatalf("写入 go.mod 失败: %v", err)
	}

	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	if output, err := tidyCmd.CombinedOutput(); err != nil {
		t.Fatalf("运行时测试 go mod tidy 失败: %v\n%s", err, output)
	}

	// 运行生成代码的测试
	testCmd := exec.Command("go", "test", "-race", "-count=1", "-v", "./runtime/...")
	testCmd.Dir = tmpDir
	output, err := testCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("运行时行为测试失败: %v\n%s", err, output)
	}
	t.Logf("运行时行为测试输出:\n%s", output)
}
