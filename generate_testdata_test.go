package main

// 本文件存放 generate_test.go 使用的 proto 测试定义，
// 按功能分组，与测试逻辑分离以提高可维护性。

// ── 基础场景 ──────────────────────────────────────────────

const testCustomEmptyProto = `syntax = "proto3";

package test.v1;

option go_package = "example.com/test/v1;testv1";

message Empty {
	string value = 1;
}

message Reply {
	string value = 1;
}

service EmptyService {
	// @summary 自定义 Empty 请求
	rpc Ping(Empty) returns (Reply);
}
`

const testStableOutputProto = `syntax = "proto3";

package test.v1;

option go_package = "example.com/test/v1;testv1";

import "google/api/annotations.proto";

message PingRequest {
	string id = 1;
}

message PingReply {
	string value = 1;
}

service PingService {
	// @summary 健康检查
	rpc Ping(PingRequest) returns (PingReply);

	// @summary 详情
	rpc GetItem(PingRequest) returns (PingReply) {
		option (google.api.http) = {
			get: "/v1/items/{id}"
		};
	}
}
`

const testMissingSummaryProto = `syntax = "proto3";

package test.v1;

option go_package = "example.com/test/v1;testv1";

message PingRequest {
	string value = 1;
}

message PingReply {
	string value = 1;
}

service SummaryService {
	// 这个方法没有接口摘要注释
	rpc Ping(PingRequest) returns (PingReply);
}
`

const testStreamingProto = `syntax = "proto3";

package test.v1;

option go_package = "example.com/test/v1;testv1";

message StreamRequest {
	string value = 1;
}

message StreamReply {
	string value = 1;
}

service StreamService {
	// @summary 流式接口
	rpc Watch(StreamRequest) returns (stream StreamReply);
}
`

// ── 路径模板 ──────────────────────────────────────────────

const testPathTemplateProto = `syntax = "proto3";

package test.v1;

option go_package = "example.com/test/v1;testv1";

import "google/api/annotations.proto";

message GetRequest {
	string id = 1;
}

message GetReply {
	string value = 1;
}

service PathService {
	// @summary 获取详情
	rpc Get(GetRequest) returns (GetReply) {
		option (google.api.http) = {
			get: "/v1/items/{id=*}"
		};
	}
}
`

const testComplexPathTemplateProto = `syntax = "proto3";

package test.v1;

option go_package = "example.com/test/v1;testv1";

import "google/api/annotations.proto";

message GetRequest {
	string name = 1;
}

message GetReply {
	string value = 1;
}

service PathService {
	// @summary 获取详情
	rpc Get(GetRequest) returns (GetReply) {
		option (google.api.http) = {
			get: "/v1/{name=projects/*/items/*}"
		};
	}
}
`

const testDefaultEmptyPathProto = `syntax = "proto3";

package test.v1;

option go_package = "example.com/test/v1;testv1";

message Req { string id = 1; }
message Reply { string value = 1; }

service EmptyPathService {
	// @summary 空路径测试
	rpc Get(Req) returns (Reply);
}
`

// ── body 绑定 ──────────────────────────────────────────────

const testBodyQueryOnlyProto = `syntax = "proto3";

package test.v1;

option go_package = "example.com/test/v1;testv1";

import "google/api/annotations.proto";

message CreateRequest {
	string id = 1;
	string keyword = 2;
	string content = 3;
}

message CreateReply {
	string value = 1;
}

service BodyService {
	// @summary 仅参数绑定
	rpc Create(CreateRequest) returns (CreateReply) {
		option (google.api.http) = {
			post: "/v1/items/{id}"
		};
	}
}
`

const testBodyFullProto = `syntax = "proto3";

package test.v1;

option go_package = "example.com/test/v1;testv1";

import "google/api/annotations.proto";

message UpdateRequest {
	string id = 1;
	string content = 2;
}

message UpdateReply {
	string value = 1;
}

service BodyService {
	// @summary 全量绑定
	rpc Update(UpdateRequest) returns (UpdateReply) {
		option (google.api.http) = {
			put: "/v1/items/{id}"
			body: "*"
		};
	}
}
`

// ── response_body ──────────────────────────────────────────

const testResponseBodyProto = `syntax = "proto3";

package test.v1;

option go_package = "example.com/test/v1;testv1";

import "google/api/annotations.proto";

message GetRequest {
	string id = 1;
}

message GetReply {
	string value = 1;
}

service HttpRuleService {
	// @summary 获取详情
	rpc Get(GetRequest) returns (GetReply) {
		option (google.api.http) = {
			get: "/v1/items/{id}"
			response_body: "value"
		};
	}
}
`

// ── additional_bindings ────────────────────────────────────

const testAdditionalBindingsProto = `syntax = "proto3";

package test.v1;

option go_package = "example.com/test/v1;testv1";

import "google/api/annotations.proto";

message MultiRequest {
	string id = 1;
}

message MultiReply {
	string value = 1;
}

service MultiService {
	// @summary 多路由接口
	rpc Get(MultiRequest) returns (MultiReply) {
		option (google.api.http) = {
			get: "/v1/items/{id}"
			additional_bindings {
				get: "/v1/items/by-id/{id}"
			}
		};
	}
}
`

const testDuplicateRouteProto = `syntax = "proto3";

package test.v1;

option go_package = "example.com/test/v1;testv1";

import "google/api/annotations.proto";

message Req {
	string id = 1;
}

message Reply {
	string value = 1;
}

service DuplicateService {
	// @summary 获取A
	rpc GetA(Req) returns (Reply) {
		option (google.api.http) = {
			get: "/v1/items/{id}"
		};
	}
	// @summary 获取B
	rpc GetB(Req) returns (Reply) {
		option (google.api.http) = {
			get: "/v1/items/{id}"
		};
	}
}
`

// ── 跨包类型 ──────────────────────────────────────────────

const testExternalTypesProto = `syntax = "proto3";

package test.v1;

option go_package = "example.com/test/v1;testv1";

import "deps/common/common.proto";

service ExternalService {
	// @summary 外部类型测试
	rpc Sync(dep.common.ExternalRequest) returns (dep.common.ExternalReply);
}
`

const testExternalCommonProto = `syntax = "proto3";

package dep.common;

option go_package = "example.com/external/common;pbx";

message ExternalRequest {
	string value = 1;
}

message ExternalReply {
	string value = 1;
}
`

// ── 特殊响应模式 ──────────────────────────────────────────

const testFileDownloadProto = `syntax = "proto3";

package test.v1;

option go_package = "example.com/test/v1;testv1";

message DownloadRequest {
	string id = 1;
}

message DownloadReply {
	bytes content = 1;
	string filename = 2;
	string content_type = 3;
}

service DownloadService {
	// @summary 下载文件
	rpc Download(DownloadRequest) returns (DownloadReply);
}
`

const testRedirectProto = `syntax = "proto3";

package test.v1;

option go_package = "example.com/test/v1;testv1";

message RedirectRequest {
	string token = 1;
}

message RedirectReply {
	string redirect_url = 1;
}

service RedirectService {
	// @summary 跳转
	rpc Redirect(RedirectRequest) returns (RedirectReply);
}
`

const testResponseModeNormalProto = `syntax = "proto3";

package test.v1;

option go_package = "example.com/test/v1;testv1";

import "google/protobuf/descriptor.proto";

extend google.protobuf.MethodOptions {
	int32 response_mode = 50008;
}

message DownloadLikeRequest {
	string id = 1;
}

message DownloadLikeReply {
	bytes content = 1;
	string filename = 2;
	string content_type = 3;
}

service DownloadLikeService {
	// @summary 普通响应模式
	rpc GetItem(DownloadLikeRequest) returns (DownloadLikeReply) {
		option (response_mode) = 1;
	}
}
`

const testResponseModeFileDownloadInvalidProto = `syntax = "proto3";

package test.v1;

option go_package = "example.com/test/v1;testv1";

import "google/protobuf/descriptor.proto";

extend google.protobuf.MethodOptions {
	int32 response_mode = 50008;
}

message DownloadRequest {
	string id = 1;
}

message DownloadReply {
	bytes content = 1;
	string filename = 2;
}

service DownloadService {
	// @summary 文件下载
	rpc Download(DownloadRequest) returns (DownloadReply) {
		option (response_mode) = 2;
	}
}
`

// ── Auth Cookie ───────────────────────────────────────────

const testAuthCookieProto = `syntax = "proto3";

package test.v1;

option go_package = "example.com/test/v1;testv1";

import "google/protobuf/descriptor.proto";

extend google.protobuf.MethodOptions {
	bool set_auth_cookie = 50001;
}

message LoginRequest {
	string username = 1;
	string password = 2;
}

message LoginReply {
	string access_token = 1;
	int64 expires_in = 2;
}

service AuthService {
	// @summary 登录
	rpc Login(LoginRequest) returns (LoginReply) {
		option (set_auth_cookie) = true;
	}
}
`

const testAuthCookieMissingFieldsProto = `syntax = "proto3";

package test.v1;

option go_package = "example.com/test/v1;testv1";

import "google/protobuf/descriptor.proto";

extend google.protobuf.MethodOptions {
	bool set_auth_cookie = 50001;
}

message LoginRequest {
	string username = 1;
}

message LoginReply {
	string token = 1;
}

service AuthService {
	// @summary 登录
	rpc Login(LoginRequest) returns (LoginReply) {
		option (set_auth_cookie) = true;
	}
}
`

// ── 权限选项 ──────────────────────────────────────────────

const testPermissionOptionsProto = `syntax = "proto3";

package test.options;

option go_package = "example.com/test/options;options";

import "google/protobuf/descriptor.proto";

extend google.protobuf.MethodOptions {
	string permission = 50002;
	string permissions = 50003;
	string any_permission = 50004;
	bool public = 50005;
	bool auth_only = 50006;
}
`

const testPermConflictAuthOnlyPermissionProto = `syntax = "proto3";

package test.v1;

option go_package = "example.com/test/v1;testv1";

import "options/options.proto";

message Req { string id = 1; }
message Reply { string value = 1; }

service ConflictService {
	// @summary 冲突测试
	rpc Get(Req) returns (Reply) {
		option (test.options.auth_only) = true;
		option (test.options.permission) = "perm.read";
	}
}
`

const testPermConflictPermissionsAnyPermProto = `syntax = "proto3";

package test.v1;

option go_package = "example.com/test/v1;testv1";

import "options/options.proto";

message Req { string id = 1; }
message Reply { string value = 1; }

service ConflictService {
	// @summary 冲突测试
	rpc Get(Req) returns (Reply) {
		option (test.options.permissions) = "a,b";
		option (test.options.any_permission) = "x,y";
	}
}
`

const testPermBlankPermissionProto = `syntax = "proto3";

package test.v1;

option go_package = "example.com/test/v1;testv1";

import "options/options.proto";

message Req { string id = 1; }
message Reply { string value = 1; }

service BlankService {
	// @summary 空白权限测试
	rpc Get(Req) returns (Reply) {
		option (test.options.permission) = "   ";
	}
}
`

const testPermBlankPermissionsProto = `syntax = "proto3";

package test.v1;

option go_package = "example.com/test/v1;testv1";

import "options/options.proto";

message Req { string id = 1; }
message Reply { string value = 1; }

service BlankService {
	// @summary 空白权限列表测试
	rpc Get(Req) returns (Reply) {
		option (test.options.permissions) = " , ";
	}
}
`

const testPermMixedEmptyItemsProto = `syntax = "proto3";

package test.v1;

option go_package = "example.com/test/v1;testv1";

import "options/options.proto";

message Req { string id = 1; }
message Reply { string value = 1; }

service MixedService {
	// @summary 混合空项权限测试
	rpc GetItems(Req) returns (Reply) {
		option (test.options.permissions) = "perm.a, , perm.b";
	}
}
`

const testAnyPermMixedEmptyItemsProto = `syntax = "proto3";

package test.v1;

option go_package = "example.com/test/v1;testv1";

import "options/options.proto";

message Req { string id = 1; }
message Reply { string value = 1; }

service MixedService {
	// @summary 混合空项任意权限测试
	rpc GetItems(Req) returns (Reply) {
		option (test.options.any_permission) = "perm.x, , perm.y";
	}
}
`

// ── 编译验证 ──────────────────────────────────────────────

const compileMainProto = `syntax = "proto3";

package test.compile.v1;

option go_package = "example.com/compile;compilev1";

import "google/api/annotations.proto";

import "google/protobuf/empty.proto";
import "deps/common/common.proto";

message CreateRequest {
	string id = 1;
	string keyword = 2;
}

service CompileService {
	// @summary 编译验证
	rpc Create(deps.common.ExternalRequest) returns (google.protobuf.Empty) {
		option (google.api.http) = {
			post: "/v1/items/{id}"
			body: "*"
			additional_bindings {
				get: "/v1/items/{id=*}"
			}
		};
	}
}
`

const compileExternalProto = `syntax = "proto3";

package deps.common;

option go_package = "example.com/compile/deps/common;commonpb";

message ExternalRequest {
	string value = 1;
}
`
