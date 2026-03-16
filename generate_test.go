package main

import (
	"strings"
	"testing"
)

func TestGenerate_ResponseStyleWrapped(t *testing.T) {
	generated := runGenerate(t, []string{"paths=source_relative", "response_style=wrapped"})
	if !strings.Contains(generated, "SuccessResponse{") {
		t.Fatalf("wrapped 风格应使用 SuccessResponse 结构体，实际输出:\n%s", generated)
	}
	if !strings.Contains(generated, "Data: data,") {
		t.Fatalf("wrapped 风格应设置 Data 字段，实际输出:\n%s", generated)
	}
}

func TestGenerate_ResponseStyleDirect(t *testing.T) {
	generated := runGenerate(t, []string{"paths=source_relative", "response_style=direct"})
	if !strings.Contains(generated, "return ctx.JSON(200, data)") {
		t.Fatalf("direct 风格应直接返回 JSON，实际输出:\n%s", generated)
	}
	if strings.Contains(generated, `"data": data`) {
		t.Fatalf("direct 风格不应生成包装 data 字段，实际输出:\n%s", generated)
	}
}

func TestGenerate_NoProjectSpecificDependencyForUpload(t *testing.T) {
	generated := runGenerate(t, []string{"paths=source_relative"})
	if strings.Contains(generated, "internal/middleware") {
		t.Fatalf("生成代码不应依赖项目私有 middleware 包，实际输出:\n%s", generated)
	}
	if strings.Contains(generated, "middleware.SetEchoContext") {
		t.Fatalf("upload 场景不应调用项目私有 SetEchoContext，实际输出:\n%s", generated)
	}
}

func TestGenerate_RegisterAcceptsOptionalWrapper(t *testing.T) {
	generated := runGenerate(t, []string{"paths=source_relative"})
	if !strings.Contains(generated, "wrappers ...UploadServiceResponseWrapper") {
		t.Fatalf("Register 应支持可选响应包装器注入，实际输出:\n%s", generated)
	}
}

func TestGenerate_CustomEmptyDoesNotUseEmptypb(t *testing.T) {
	generated := runGenerateProto(t, testCustomEmptyProto, []string{"paths=source_relative"})
	if strings.Contains(generated, "emptypb.Empty") {
		t.Fatalf("自定义 Empty 不应被识别为 emptypb.Empty，实际输出:\n%s", generated)
	}
	if !strings.Contains(generated, "Ping(context.Context, *Empty) (*Reply, error)") {
		t.Fatalf("自定义 Empty 请求类型应保留为当前包 Empty，实际输出:\n%s", generated)
	}
	if !strings.Contains(generated, "var in Empty") {
		t.Fatalf("自定义 Empty 应保留正常绑定逻辑，实际输出:\n%s", generated)
	}
}

func TestGenerate_PathTemplateStarNormalizesToEchoParam(t *testing.T) {
	generated := runGenerateProto(t, testPathTemplateProto, []string{"paths=source_relative"})
	if !strings.Contains(generated, `s.router.Add("GET", "/v1/items/:id", s.Get_0)`) {
		t.Fatalf("{id=*} 应转换为 Echo 路径参数 :id，实际输出:\n%s", generated)
	}
}

func TestGenerate_RejectsComplexPathTemplate(t *testing.T) {
	_, errOutput := runGenerateProtoExpectError(t, testComplexPathTemplateProto, []string{"paths=source_relative"})
	if !strings.Contains(errOutput, "暂不支持复杂路径模板") {
		t.Fatalf("复杂路径模板应显式报错，实际输出:\n%s", errOutput)
	}
}

func TestGenerate_ResponseBodySelectsSubField(t *testing.T) {
	code := runGenerateProto(t, testResponseBodyProto, []string{"paths=source_relative"})
	// response_body="value" 时，应返回 out.Value 而非 out
	if !strings.Contains(code, "out.Value") {
		t.Fatalf("response_body 应生成 out.Value，实际输出:\n%s", code)
	}
	if !strings.Contains(code, "s.resp.Success(ctx, out.Value)") {
		t.Fatalf("response_body 应调用 s.resp.Success(ctx, out.Value)，实际输出:\n%s", code)
	}
}

func TestGenerate_ResponseBodyFieldNotFoundReportsError(t *testing.T) {
	proto := `syntax = "proto3";
package test.v1;
option go_package = "example.com/test/v1;testv1";

import "google/api/annotations.proto";

service Svc {
  // @summary 测试
  rpc Get(GetReq) returns (GetReply) {
    option (google.api.http) = {
      get: "/items/{id}"
      response_body: "nonexistent"
    };
  }
}

message GetReq {
  string id = 1;
}

message GetReply {
  string value = 1;
}
`
	_, errOutput := runGenerateProtoExpectError(t, proto, []string{"paths=source_relative"})
	if !strings.Contains(errOutput, "未找到对应字段") {
		t.Fatalf("response_body 引用不存在的字段应报错，实际输出:\n%s", errOutput)
	}
}

func TestGenerate_ResponseBodyWithFileDownloadReportsError(t *testing.T) {
	proto := `syntax = "proto3";
package test.v1;
option go_package = "example.com/test/v1;testv1";

import "google/api/annotations.proto";

service Svc {
  // @summary 测试
  rpc Download(GetReq) returns (DownloadReply) {
    option (google.api.http) = {
      get: "/items/{id}"
      response_body: "content"
    };
  }
}

message GetReq {
  string id = 1;
}

message DownloadReply {
  bytes content = 1;
  string filename = 2;
  string content_type = 3;
}
`
	_, errOutput := runGenerateProtoExpectError(t, proto, []string{"paths=source_relative"})
	if !strings.Contains(errOutput, "response_body") || !strings.Contains(errOutput, "文件下载") {
		t.Fatalf("response_body 与文件下载并存应报冲突，实际输出:\n%s", errOutput)
	}
}

func TestGenerate_RejectsStreamingRPC(t *testing.T) {
	_, errOutput := runGenerateProtoExpectError(t, testStreamingProto, []string{"paths=source_relative"})
	if !strings.Contains(errOutput, "暂不支持 streaming RPC") {
		t.Fatalf("streaming RPC 应显式报错，实际输出:\n%s", errOutput)
	}
}

func TestGenerate_OutputIsStableAcrossRuns(t *testing.T) {
	first := runGenerateProto(t, testStableOutputProto, []string{"paths=source_relative"})
	second := runGenerateProto(t, testStableOutputProto, []string{"paths=source_relative"})
	if first != second {
		t.Fatalf("同一输入的生成结果应稳定一致")
	}
	if !strings.Contains(first, "GetItem(context.Context, *PingRequest) (*PingReply, error)") {
		t.Fatalf("接口方法应按稳定顺序输出，实际输出:\n%s", first)
	}
}

func TestGenerate_UsesGoPackageAliasForExternalTypes(t *testing.T) {
	generated := runGenerateProtoWithExtras(t, testExternalTypesProto, map[string]string{
		"deps/common/common.proto": testExternalCommonProto,
	}, []string{"paths=source_relative"})
	if !strings.Contains(generated, `pbx "example.com/external/common"`) {
		t.Fatalf("跨包类型应使用 go_package 指定的包名 pbx，实际输出:\n%s", generated)
	}
	if !strings.Contains(generated, "Sync(context.Context, *pbx.ExternalRequest) (*pbx.ExternalReply, error)") {
		t.Fatalf("跨包类型应使用 pbx 前缀限定，实际输出:\n%s", generated)
	}
}

func TestGenerate_HTTPBodyEmptyUsesPathAndQueryBinding(t *testing.T) {
	generated := runGenerateProto(t, testBodyQueryOnlyProto, []string{"paths=source_relative"})
	if !strings.Contains(generated, "s.binder.BindPathParams(ctx, &in)") {
		t.Fatalf("body 为空时应绑定 path 参数，实际输出:\n%s", generated)
	}
	if !strings.Contains(generated, "s.binder.BindQueryParams(ctx, &in)") {
		t.Fatalf("body 为空时应绑定 query 参数，实际输出:\n%s", generated)
	}
	if strings.Contains(generated, "s.binder.BindBody(ctx, &in)") {
		t.Fatalf("body 为空时不应绑定请求体，实际输出:\n%s", generated)
	}
	if strings.Contains(generated, "ctx.Bind(&in)") {
		t.Fatalf("显式 http rule 的 body 为空场景不应回退到 ctx.Bind，实际输出:\n%s", generated)
	}
}

func TestGenerate_HTTPBodyStarUsesPathAndBodyBinding(t *testing.T) {
	generated := runGenerateProto(t, testBodyFullProto, []string{"paths=source_relative"})
	if !strings.Contains(generated, "s.binder.BindPathParams(ctx, &in)") {
		t.Fatalf("body=* 时应绑定 path 参数，实际输出:\n%s", generated)
	}
	if !strings.Contains(generated, "s.binder.BindBody(ctx, &in)") {
		t.Fatalf("body=* 时应绑定请求体，实际输出:\n%s", generated)
	}
	if strings.Contains(generated, "s.binder.BindQueryParams(ctx, &in)") {
		t.Fatalf("body=* 时不应绑定 query 参数，实际输出:\n%s", generated)
	}
}

func TestGenerate_PermissionMetaUsesUniqueHandlerName(t *testing.T) {
	generated := runGenerateProto(t, testAdditionalBindingsProto, []string{"paths=source_relative"})
	if !strings.Contains(generated, `Handler:     "Get_0"`) {
		t.Fatalf("PermissionMeta 应使用唯一 HandlerName，实际输出:\n%s", generated)
	}
	if !strings.Contains(generated, `Handler:     "Get_1"`) {
		t.Fatalf("additional_bindings 也应生成唯一 HandlerName，实际输出:\n%s", generated)
	}
}

func TestGenerate_InvalidResponseStyleReportsError(t *testing.T) {
	_, errOutput := runGenerateProtoExpectError(t, testStableOutputProto, []string{"paths=source_relative", "response_style=invalid"})
	if !strings.Contains(errOutput, "invalid response_style") {
		t.Fatalf("无效 response_style 应报错，实际输出:\n%s", errOutput)
	}
}

func TestGenerate_MissingSummaryReportsError(t *testing.T) {
	_, errOutput := runGenerateProtoExpectError(t, testMissingSummaryProto, []string{"paths=source_relative"})
	if !strings.Contains(errOutput, "缺少 @summary 注释") {
		t.Fatalf("缺少 @summary 应报错，实际输出:\n%s", errOutput)
	}
}

func TestGenerate_DuplicateRouteReportsError(t *testing.T) {
	_, errOutput := runGenerateProtoExpectError(t, testDuplicateRouteProto, []string{"paths=source_relative"})
	if !strings.Contains(errOutput, "检测到重复路由") {
		t.Fatalf("重复路由应报错，实际输出:\n%s", errOutput)
	}
}

func TestGenerate_FileDownloadResponse(t *testing.T) {
	generated := runGenerateProto(t, testFileDownloadProto, []string{"paths=source_relative"})
	if !strings.Contains(generated, "ctx.Blob(200") {
		t.Fatalf("文件下载响应应使用 ctx.Blob，实际输出:\n%s", generated)
	}
	if !strings.Contains(generated, "Content-Disposition") {
		t.Fatalf("文件下载响应应设置 Content-Disposition，实际输出:\n%s", generated)
	}
}

func TestGenerate_RedirectResponse(t *testing.T) {
	generated := runGenerateProto(t, testRedirectProto, []string{"paths=source_relative"})
	if !strings.Contains(generated, "ctx.Redirect(302") {
		t.Fatalf("重定向响应应使用 ctx.Redirect，实际输出:\n%s", generated)
	}
}

func TestGenerate_OutputCompiles(t *testing.T) {
	bodyFieldProto := `syntax = "proto3";
package test.v1;
option go_package = "example.com/compile/testv1;testv1";

import "google/api/annotations.proto";

service Users {
  // @summary 创建用户
  rpc CreateUser(CreateUserRequest) returns (CreateUserReply) {
    option (google.api.http) = {
      post: "/users/{parent}"
      body: "user"
    };
  }
}

message User {
  string name = 1;
  string email = 2;
}

message CreateUserRequest {
  string parent = 1;
  User user = 2;
}

message CreateUserReply {
  User user = 1;
}
`
	responseBodyProto := `syntax = "proto3";
package test.v1;
option go_package = "example.com/compile/testv1;testv1";

import "google/api/annotations.proto";

service Items {
  // @summary 获取详情
  rpc Get(GetReq) returns (GetReply) {
    option (google.api.http) = {
      get: "/items/{id}"
      response_body: "value"
    };
  }
}

message GetReq {
  string id = 1;
}

message GetReply {
  string value = 1;
}
`
	responseModeNormalProto := `syntax = "proto3";
package test.v1;
option go_package = "example.com/compile/testv1;testv1";

import "google/protobuf/descriptor.proto";

extend google.protobuf.MethodOptions {
  int32 response_mode = 50008;
}

service DownloadLike {
  // @summary 普通模式
  rpc GetItem(GetReq) returns (GetReply) {
    option (response_mode) = 1;
  }
}

message GetReq {
  string id = 1;
}

message GetReply {
  bytes content = 1;
  string filename = 2;
  string content_type = 3;
}
`
	compileGeneratedPackages(t,
		compileCase{
			name:         "case_minimal",
			protoContent: testStableOutputProto,
		},
		compileCase{
			name:         "case_auth_cookie",
			protoContent: testAuthCookieProto,
		},
		compileCase{
			name:         "case_body_field",
			protoContent: bodyFieldProto,
		},
		compileCase{
			name:         "case_response_body",
			protoContent: responseBodyProto,
		},
		compileCase{
			name:         "case_response_mode_normal",
			protoContent: responseModeNormalProto,
		},
	)
}

func TestGenerate_ResponseModeFileDownloadRequiresDownloadFields(t *testing.T) {
	_, errOutput := runGenerateProtoExpectError(t, testResponseModeFileDownloadInvalidProto, []string{"paths=source_relative"})
	if !strings.Contains(errOutput, "response_mode=FILE_DOWNLOAD") {
		t.Fatalf("response_mode=2 且下载字段不完整应报错，实际输出:\n%s", errOutput)
	}
	if !strings.Contains(errOutput, "content_type") {
		t.Fatalf("错误信息应包含缺失字段 content_type，实际输出:\n%s", errOutput)
	}
}

func TestGenerate_ResponseModeRedirectRequiresRedirectUrl(t *testing.T) {
	proto := `syntax = "proto3";
package test.v1;
option go_package = "example.com/test/v1;testv1";

import "google/protobuf/descriptor.proto";

extend google.protobuf.MethodOptions {
  int32 response_mode = 50008;
}

service Svc {
  // @summary 测试重定向
  rpc Go(GoReq) returns (GoReply) {
    option (response_mode) = 3;
  }
}

message GoReq { string id = 1; }
message GoReply { string url = 1; }
`
	_, errOutput := runGenerateProtoExpectError(t, proto, []string{"paths=source_relative"})
	if !strings.Contains(errOutput, "response_mode=REDIRECT") {
		t.Fatalf("response_mode=REDIRECT 但缺少 redirect_url 应报错，实际输出:\n%s", errOutput)
	}
}

func TestGenerate_ResponseModeNormalOverridesFieldInference(t *testing.T) {
	// 消息结构符合文件下载推断条件，但 response_mode=NORMAL 应压过旧推断
	proto := `syntax = "proto3";
package test.v1;
option go_package = "example.com/test/v1;testv1";

import "google/protobuf/descriptor.proto";

extend google.protobuf.MethodOptions {
  int32 response_mode = 50008;
}

service Svc {
  // @summary 下载文件
  rpc GetFile(GetFileReq) returns (FileReply) {
    option (response_mode) = 1;
  }
}

message GetFileReq { string id = 1; }
message FileReply {
  bytes content = 1;
  string filename = 2;
  string content_type = 3;
}
`
	code := runGenerateProto(t, proto, []string{"paths=source_relative"})
	if strings.Contains(code, "ctx.Blob") {
		t.Error("response_mode=NORMAL 时不应生成 ctx.Blob（文件下载），即使字段结构匹配")
	}
	if strings.Contains(code, "ctx.Redirect") {
		t.Error("response_mode=NORMAL 时不应生成 ctx.Redirect")
	}
	if !strings.Contains(code, "s.resp.Success(ctx, out)") {
		t.Error("response_mode=NORMAL 时应走普通 s.resp.Success(ctx, out)")
	}
}

func TestGenerate_InvalidResponseModeReportsError(t *testing.T) {
	proto := `syntax = "proto3";
package test.v1;
option go_package = "example.com/test/v1;testv1";

import "google/protobuf/descriptor.proto";

extend google.protobuf.MethodOptions {
  int32 response_mode = 50008;
}

service Svc {
  // @summary 测试
  rpc Get(GetReq) returns (GetReply) {
    option (response_mode) = 99;
  }
}

message GetReq { string id = 1; }
message GetReply { string value = 1; }
`
	_, errOutput := runGenerateProtoExpectError(t, proto, []string{"paths=source_relative"})
	if !strings.Contains(errOutput, "无效的 response_mode") {
		t.Fatalf("非法 response_mode 值应报错，实际输出:\n%s", errOutput)
	}
}

func TestGenerate_PermConflictAuthOnlyAndPermission(t *testing.T) {
	_, errOutput := runGenerateProtoInternal(t, testPermConflictAuthOnlyPermissionProto, map[string]string{
		"options/options.proto": testPermissionOptionsProto,
	}, []string{"paths=source_relative"})
	if errOutput == "" {
		t.Fatal("auth_only 与 permission 同时设置应报错")
	}
	if !strings.Contains(errOutput, "权限选项冲突") {
		t.Fatalf("错误信息应包含'权限选项冲突'，实际:\n%s", errOutput)
	}
}

func TestGenerate_PermConflictPermissionsAndAnyPermission(t *testing.T) {
	_, errOutput := runGenerateProtoInternal(t, testPermConflictPermissionsAnyPermProto, map[string]string{
		"options/options.proto": testPermissionOptionsProto,
	}, []string{"paths=source_relative"})
	if errOutput == "" {
		t.Fatal("permissions 与 any_permission 同时设置应报错")
	}
	if !strings.Contains(errOutput, "权限选项冲突") {
		t.Fatalf("错误信息应包含'权限选项冲突'，实际:\n%s", errOutput)
	}
}

func TestGenerate_PermBlankPermissionReportsError(t *testing.T) {
	_, errOutput := runGenerateProtoInternal(t, testPermBlankPermissionProto, map[string]string{
		"options/options.proto": testPermissionOptionsProto,
	}, []string{"paths=source_relative"})
	if errOutput == "" {
		t.Fatal("纯空白 permission 应报错")
	}
	if !strings.Contains(errOutput, "permission 值不能为纯空白") {
		t.Fatalf("错误信息应包含'permission 值不能为纯空白'，实际:\n%s", errOutput)
	}
}

func TestGenerate_PermBlankPermissionsReportsError(t *testing.T) {
	_, errOutput := runGenerateProtoInternal(t, testPermBlankPermissionsProto, map[string]string{
		"options/options.proto": testPermissionOptionsProto,
	}, []string{"paths=source_relative"})
	if errOutput == "" {
		t.Fatal("全空白 permissions 应报错")
	}
	if !strings.Contains(errOutput, "permissions 值不能为纯空白或空列表") {
		t.Fatalf("错误信息应包含'permissions 值不能为纯空白或空列表'，实际:\n%s", errOutput)
	}
}

func TestGenerate_PermMixedEmptyItemsFiltered(t *testing.T) {
	generated := runGenerateProtoWithExtras(t, testPermMixedEmptyItemsProto, map[string]string{
		"options/options.proto": testPermissionOptionsProto,
	}, []string{"paths=source_relative"})
	// 验证生成的 CheckAllPermissions 权限数组精确包含有效项，无空串
	expectedArray := `[]string{"perm.a", "perm.b"}`
	if !strings.Contains(generated, expectedArray) {
		t.Fatalf("permissions 混合空项过滤后应生成 %s，实际输出:\n%s", expectedArray, generated)
	}
	// 验证 PermissionMeta 中的 Permission 展示也是规范化后的值
	if !strings.Contains(generated, `Permission:  "perm.a,perm.b"`) {
		t.Fatalf("PermissionMeta 的 Permission 展示应为规范化后的 perm.a,perm.b，实际输出:\n%s", generated)
	}
}

func TestGenerate_AnyPermMixedEmptyItemsFiltered(t *testing.T) {
	generated := runGenerateProtoWithExtras(t, testAnyPermMixedEmptyItemsProto, map[string]string{
		"options/options.proto": testPermissionOptionsProto,
	}, []string{"paths=source_relative"})
	// 验证生成的 CheckAnyPermission 权限数组精确包含有效项，无空串
	expectedArray := `[]string{"perm.x", "perm.y"}`
	if !strings.Contains(generated, expectedArray) {
		t.Fatalf("any_permission 混合空项过滤后应生成 %s，实际输出:\n%s", expectedArray, generated)
	}
	// 验证 PermissionMeta 中的 Permission 展示也是规范化后的值
	if !strings.Contains(generated, `Permission:  "perm.x,perm.y"`) {
		t.Fatalf("PermissionMeta 的 Permission 展示应为规范化后的 perm.x,perm.y，实际输出:\n%s", generated)
	}
}

func TestGenerate_DefaultEmptyPathReportsError(t *testing.T) {
	_, errOutput := runGenerateProtoExpectError(t, testDefaultEmptyPathProto, []string{"paths=source_relative"})
	if errOutput == "" {
		t.Fatal("无法推导路径的方法应报错")
	}
	if !strings.Contains(errOutput, "无法推导出有效路径") {
		t.Fatalf("错误信息应包含'无法推导出有效路径'，实际:\n%s", errOutput)
	}
}

func TestGenerate_AuthCookieSecureUsesIsProduction(t *testing.T) {
	generated := runGenerateProto(t, testAuthCookieProto, []string{"paths=source_relative"})
	// 验证生成代码中包含 IsProduction 包级别缓存变量定义
	if !strings.Contains(generated, "IsProduction = func() bool") {
		t.Fatalf("auth cookie 生成代码应包含 IsProduction 缓存变量定义，实际输出:\n%s", generated)
	}
	// 验证使用 http.Cookie 结构化设置（而非手工拼接字符串）
	if !strings.Contains(generated, "http.Cookie") {
		t.Fatalf("auth cookie 应使用 http.Cookie 结构化设置，实际输出:\n%s", generated)
	}
	// 验证 Secure 标志由缓存变量直接赋值
	if !strings.Contains(generated, "authCookie.Secure = authServiceIsProduction") {
		t.Fatalf("auth cookie Secure 应通过 IsProduction 缓存变量直接赋值，实际输出:\n%s", generated)
	}
	// 验证使用 ctx.SetCookie 而非手工拼 Set-Cookie 头
	if !strings.Contains(generated, "ctx.SetCookie(authCookie)") {
		t.Fatalf("auth cookie 应使用 ctx.SetCookie 设置，实际输出:\n%s", generated)
	}
	// 验证 SameSite 使用标准常量
	if !strings.Contains(generated, "http.SameSiteLaxMode") {
		t.Fatalf("auth cookie SameSite 应使用 http.SameSiteLaxMode 常量，实际输出:\n%s", generated)
	}
	// 验证 net/http import 存在
	if !strings.Contains(generated, `"net/http"`) {
		t.Fatalf("auth cookie 需要 net/http import，实际输出:\n%s", generated)
	}
}

func TestGenerate_AuthCookieMissingFieldsReportsError(t *testing.T) {
	_, errOutput := runGenerateProtoExpectError(t, testAuthCookieMissingFieldsProto, []string{"paths=source_relative"})
	if errOutput == "" {
		t.Fatal("set_auth_cookie 响应消息缺少 AccessToken/ExpiresIn 字段应报错")
	}
	if !strings.Contains(errOutput, "set_auth_cookie") {
		t.Fatalf("错误信息应提及 set_auth_cookie，实际:\n%s", errOutput)
	}
}

func TestGenerate_NoNetHTTPImportWithoutAuthCookie(t *testing.T) {
	generated := runGenerateProto(t, testStableOutputProto, []string{"paths=source_relative"})
	if strings.Contains(generated, `"net/http"`) {
		t.Fatalf("无 auth cookie 场景不应 import net/http，实际输出:\n%s", generated)
	}
}

func TestGenerate_GRPCStatusMappingPresent(t *testing.T) {
	code := runGenerateProto(t, testStableOutputProto, []string{"paths=source_relative"})

	// 验证 gRPC status/codes import 存在
	if !strings.Contains(code, `grpcCodes "google.golang.org/grpc/codes"`) {
		t.Error("生成代码应包含 grpcCodes import")
	}
	if !strings.Contains(code, `grpcStatus "google.golang.org/grpc/status"`) {
		t.Error("生成代码应包含 grpcStatus import")
	}

	// 验证映射函数存在
	if !strings.Contains(code, "GRPCCodeToHTTPStatus") {
		t.Error("生成代码应包含 GRPCCodeToHTTPStatus 映射函数")
	}

	// 验证 Error 方法中调用了 grpcStatus.FromError
	if !strings.Contains(code, "grpcStatus.FromError(err)") {
		t.Error("生成代码的默认 Error 方法应调用 grpcStatus.FromError")
	}

	// 验证关键映射条目
	for _, keyword := range []string{
		"grpcCodes.InvalidArgument",
		"grpcCodes.Unauthenticated",
		"grpcCodes.PermissionDenied",
		"grpcCodes.NotFound",
		"grpcCodes.AlreadyExists",
		"grpcCodes.ResourceExhausted",
		"grpcCodes.Unimplemented",
		"grpcCodes.Unavailable",
		"grpcCodes.DeadlineExceeded",
	} {
		if !strings.Contains(code, keyword) {
			t.Errorf("生成代码应包含映射条目 %s", keyword)
		}
	}
}

func TestGenerate_BodyFieldBinding(t *testing.T) {
	proto := `syntax = "proto3";
package test.v1;
option go_package = "example.com/test/v1;testv1";

import "google/api/annotations.proto";

service Users {
  // @summary 创建用户
  rpc CreateUser(CreateUserRequest) returns (CreateUserReply) {
    option (google.api.http) = {
      post: "/users/{parent}"
      body: "user"
    };
  }
}

message User {
  string name = 1;
  string email = 2;
}

message CreateUserRequest {
  string parent = 1;
  User user = 2;
}

message CreateUserReply {
  User user = 1;
}
`
	code := runGenerateProto(t, proto, []string{"paths=source_relative"})

	// 验证使用了 body field 绑定模式
	if !strings.Contains(code, "in.User == nil") {
		t.Error("body field 绑定应包含 nil 检查 in.User == nil")
	}
	if !strings.Contains(code, "in.User = new(") {
		t.Error("body field 绑定应包含 new() 初始化")
	}
	if !strings.Contains(code, "s.binder.BindBody(ctx, in.User)") {
		t.Error("body field 绑定应调用 s.binder.BindBody(ctx, in.User)")
	}
	// 验证仍有 path/query 绑定
	if !strings.Contains(code, "s.binder.BindPathParams(ctx, &in)") {
		t.Error("body field 绑定应包含 BindPathParams")
	}
	if !strings.Contains(code, "s.binder.BindQueryParams(ctx, &in)") {
		t.Error("body field 绑定应包含 BindQueryParams")
	}
}

func TestGenerate_BodyFieldNotFoundReportsError(t *testing.T) {
	proto := `syntax = "proto3";
package test.v1;
option go_package = "example.com/test/v1;testv1";

import "google/api/annotations.proto";

service Users {
  // @summary 创建用户
  rpc CreateUser(CreateUserRequest) returns (CreateUserReply) {
    option (google.api.http) = {
      post: "/users"
      body: "nonexistent"
    };
  }
}

message CreateUserRequest {
  string name = 1;
}

message CreateUserReply {
  string id = 1;
}
`
	_, errOutput := runGenerateProtoExpectError(t, proto, []string{"paths=source_relative"})
	if !strings.Contains(errOutput, "未找到对应字段") {
		t.Fatalf("body 引用不存在的字段应报错，实际输出:\n%s", errOutput)
	}
}

func TestGenerate_BodyFieldScalarReportsError(t *testing.T) {
	proto := `syntax = "proto3";
package test.v1;
option go_package = "example.com/test/v1;testv1";

import "google/api/annotations.proto";

service Users {
  // @summary 创建用户
  rpc CreateUser(CreateUserRequest) returns (CreateUserReply) {
    option (google.api.http) = {
      post: "/users"
      body: "name"
    };
  }
}

message CreateUserRequest {
  string name = 1;
}

message CreateUserReply {
  string id = 1;
}
`
	_, errOutput := runGenerateProtoExpectError(t, proto, []string{"paths=source_relative"})
	if !strings.Contains(errOutput, "仅支持 message 类型字段") {
		t.Fatalf("body 指向标量字段应报错，实际输出:\n%s", errOutput)
	}
}

func TestGenerate_BodyFieldOneofReportsError(t *testing.T) {
	proto := `syntax = "proto3";
package test.v1;
option go_package = "example.com/test/v1;testv1";

import "google/api/annotations.proto";

service Users {
  // @summary 创建用户
  rpc Create(CreateReq) returns (CreateReply) {
    option (google.api.http) = {
      post: "/users"
      body: "user"
    };
  }
}

message User {
  string name = 1;
}

message Admin {
  string name = 1;
}

message CreateReq {
  oneof payload {
    User user = 1;
    Admin admin = 2;
  }
}

message CreateReply {
  string id = 1;
}
`
	_, errOutput := runGenerateProtoExpectError(t, proto, []string{"paths=source_relative"})
	if !strings.Contains(errOutput, "oneof") {
		t.Fatalf("body 指向 oneof 成员应报错，实际输出:\n%s", errOutput)
	}
}

func TestGenerate_ResponseBodyOneofReportsError(t *testing.T) {
	proto := `syntax = "proto3";
package test.v1;
option go_package = "example.com/test/v1;testv1";

import "google/api/annotations.proto";

service Users {
  // @summary 获取用户
  rpc Get(GetReq) returns (GetReply) {
    option (google.api.http) = {
      get: "/users/{id}"
      response_body: "user"
    };
  }
}

message GetReq {
  string id = 1;
}

message User {
  string name = 1;
}

message Admin {
  string name = 1;
}

message GetReply {
  oneof payload {
    User user = 1;
    Admin admin = 2;
  }
}
`
	_, errOutput := runGenerateProtoExpectError(t, proto, []string{"paths=source_relative"})
	if !strings.Contains(errOutput, "oneof") {
		t.Fatalf("response_body 指向 oneof 成员应报错，实际输出:\n%s", errOutput)
	}
}

func TestGenerate_ValidatorGuardPresent(t *testing.T) {
	generated := runGenerateProto(t, testStableOutputProto, []string{"paths=source_relative"})
	if !strings.Contains(generated, "ctx.Echo().Validator != nil") {
		t.Fatalf("生成代码应包含 Validator != nil 守卫，实际输出:\n%s", generated)
	}
}

func TestGenerate_HopByHopHeaderFilterPresent(t *testing.T) {
	generated := runGenerateProto(t, testStableOutputProto, []string{"paths=source_relative"})
	if !strings.Contains(generated, "SkipHeaders") {
		t.Fatalf("生成代码应包含 SkipHeaders 过滤逻辑，实际输出:\n%s", generated)
	}
	// 验证包含关键的 hop-by-hop 头
	for _, header := range []string{"Connection", "Transfer-Encoding", "Upgrade"} {
		if !strings.Contains(generated, `"`+header+`"`) {
			t.Errorf("SkipHeaders 应包含 %s", header)
		}
	}
}

func TestGenerate_SetResponseHeaderHelperPresent(t *testing.T) {
	generated := runGenerateProto(t, testStableOutputProto, []string{"paths=source_relative"})
	if !strings.Contains(generated, "func SetPingServiceResponseHeader(ctx context.Context, key, value string)") {
		t.Fatalf("生成代码应包含导出的 SetXxxResponseHeader helper 函数，实际输出:\n%s", generated)
	}
}
