# protoc-gen-go-echo

`protoc-gen-go-echo` 是一个基于 `protogen` 的 Echo HTTP 代码生成插件，用于从 `proto service` 生成：

- `XxxHTTPServer` 业务接口
- `RegisterXxxHTTPServer` 路由注册函数
- 权限检查器接口与权限元信息
- 统一响应封装（可选覆盖）

## 安装

```bash
go install github.com/orzmoe/protoc-gen-go-echo@latest
```

或在仓库内构建：

```bash
make build
```

## 使用

```bash
protoc \
  -I . \
  --go-echo_out=. \
  --go-echo_opt=paths=source_relative \
  --go-echo_opt=response_style=wrapped \
  api.proto
```

### 插件参数

- `paths=source_relative`：生成文件与 proto 相对路径对齐。
- `module=<prefix>`：去除 `go_package` 路径中的指定前缀，按剩余路径输出文件。适用于 monorepo 中生成路径需要与项目目录对齐的场景。例如：
  ```bash
  --go-echo_opt=module=github.com/your-org/your-repo
  ```
  > `module` 是 `protogen` 内置支持的参数，与 `--go_opt=module=...` 行为一致。
- `response_style=wrapped|direct`：
  - `wrapped`（默认）：成功响应为 `{code,msg,data}`。
  - `direct`：成功响应直接 `ctx.JSON(200, data)`。

> 参数遵循 protoc 插件约定：`--<plugin>_opt=<k=v>`。本插件名为 `go-echo`，因此使用 `--go-echo_opt=...`。参考：
> - https://protobuf.dev/reference/go/go-generated/#invocation

## 生成代码中的扩展点

### 1) 可选响应包装器注入

生成的注册函数签名：

```go
func RegisterXxxHTTPServer(e *echo.Group, srv XxxHTTPServer, perm XxxPermissionChecker, wrappers ...XxxResponseWrapper)
```

- 不传 `wrappers`：使用内置默认响应实现。
- 传入一个 `XxxResponseWrapper`：覆盖默认成功/错误/参数错误响应逻辑。

### 2) Upload 场景 Echo Context 注入

upload 方法会把 `echo.Context` 写入 `context.Context`，并生成读取函数：

```go
func GetXxxEchoContext(ctx context.Context) (echo.Context, bool)
```

这样可以在业务层安全获取 `echo.Context`，无需依赖任意项目私有 middleware。

## google.api.http 支持

### body 字段绑定

`body` 支持三种模式：

| body 值 | 绑定行为 |
|---------|---------|
| 空（未设置） | 只绑定 path + query 参数 |
| `*` | path 参数 + 请求体绑定到整个 request message |
| `"<field>"` | path + query 绑定到 request 其他字段，请求体绑定到指定子消息 |

示例：
```protobuf
rpc CreateUser(CreateUserRequest) returns (User) {
  option (google.api.http) = {
    post: "/users/{parent}"
    body: "user"  // 请求体只绑定到 CreateUserRequest.user 字段
  };
}
```

> 第一版 `body` 具体字段仅支持**顶层 message 类型字段**，不支持标量、repeated 或 map 字段。

### response_body 字段选择

`response_body` 允许只返回响应消息的某个子字段：

```protobuf
rpc GetItem(GetItemReq) returns (GetItemResp) {
  option (google.api.http) = {
    get: "/items/{id}"
    response_body: "item"  // 只返回 GetItemResp.item，不返回整个 resp
  };
}
```

> 第一版仅支持顶层字段。`response_body` 不能与文件下载或重定向同时使用。

## 特殊响应类型

插件支持两种方式标记特殊响应类型：**显式 option**（推荐）和**字段推断**（兼容模式）。

### 显式 response_mode option（推荐）

通过 `extend google.protobuf.MethodOptions` 中的 `response_mode` 字段显式指定响应类型：

```protobuf
extend google.protobuf.MethodOptions {
  // 0=未指定（走字段推断）, 1=普通, 2=文件下载, 3=重定向
  int32 response_mode = 50008;
}

rpc DownloadReport(Req) returns (FileResp) {
  option (response_mode) = 2;  // FILE_DOWNLOAD
}

rpc RedirectToSSO(Req) returns (RedirectResp) {
  option (response_mode) = 3;  // REDIRECT
}
```

使用显式 option 时，插件会校验响应消息结构是否匹配指定模式。

### 字段推断（兼容模式）

未设置 `response_mode` 时，插件会按以下规则自动推断：

#### 文件下载

当响应消息**同时包含**以下三个字段时，自动生成文件下载逻辑（`ctx.Blob`）：

| 字段名 | 类型 | 说明 |
|--------|------|------|
| `content` | `bytes` | 文件内容 |
| `filename` | `string` | 文件名 |
| `content_type` | `string` | MIME 类型 |

#### 重定向

当响应消息**仅包含一个**字段且满足以下条件时，自动生成 302 重定向：

| 字段名 | 类型 |
|--------|------|
| `redirect_url` | `string` |

### Auth Cookie（登录令牌）

当方法设置了 `set_auth_cookie = true` 选项时，生成 `Set-Cookie` 逻辑。**响应消息必须包含**：

| 字段名 | 类型 | 说明 |
|--------|------|------|
| `access_token` | `string` | Cookie 值 |
| `expires_in` | `int32`/`int64`/`uint32`/`uint64` | Cookie MaxAge（秒） |

> Cookie 属性：`HttpOnly`、`SameSite=Lax`、`Path=/`。`Secure` 标志根据环境变量 `ENV` 或 `ENVIRONMENT` 在进程启动时缓存判断（`production`/`prod` 时启用）。

## 环境变量

生成代码在**进程启动时**一次性读取以下环境变量并缓存，不会在每次请求中重复读取：

| 变量名 | 值 | 作用 |
|--------|------|------|
| `ENV` / `ENVIRONMENT` | `development`/`dev`/`local` | 开发模式：错误响应包含详细堆栈 |
| `ENV` / `ENVIRONMENT` | `production`/`prod` | 生产模式：Cookie 添加 `Secure` 标志 |
| `DETAILED_VALIDATION` | `true` | 参数错误响应包含详细校验信息 |
| `VERBOSE_SUCCESS` | `true` | 成功响应附加时间戳和 request_id（仅开发模式） |

## 错误处理

生成的默认错误响应支持两种错误类型识别，按优先级：

1. **业务错误接口**：实现 `HTTPCode() int` / `Message() string` / `Code() int` 的错误类型
2. **gRPC status**：自动将 `google.golang.org/grpc/status` 错误映射为对应 HTTP 状态码

gRPC 状态码映射表：

| gRPC Code | HTTP Status |
|-----------|-------------|
| `InvalidArgument` / `FailedPrecondition` / `OutOfRange` | 400 |
| `Unauthenticated` | 401 |
| `PermissionDenied` | 403 |
| `NotFound` | 404 |
| `AlreadyExists` / `Aborted` | 409 |
| `ResourceExhausted` | 429 |
| `Canceled` | 499 |
| `Unimplemented` | 501 |
| `Unavailable` | 503 |
| `DeadlineExceeded` | 504 |
| `Internal` / `DataLoss` / 其他 | 500 |

> 如需自定义映射逻辑，注入自定义 `ResponseWrapper` 覆盖 `Error()` 方法即可。

## 运行时约定

### Validator

生成的 handler 在参数绑定后会检查 `ctx.Echo().Validator` 是否已注册：

- **已注册**：自动调用 `ctx.Validate(&in)` 校验请求参数
- **未注册**：跳过校验，直接进入业务逻辑

如需启用参数校验，请在 Echo 实例上注册 Validator：

```go
e.Validator = &MyValidator{} // 实现 echo.Validator 接口
```

### HTTP Header 透传

生成代码会将 HTTP 请求头透传到 gRPC metadata，但会**自动过滤**以下头：

- hop-by-hop 头：`Connection`、`Keep-Alive`、`Transfer-Encoding`、`Upgrade`、`Te`、`Trailer`
- 代理头：`Proxy-Authenticate`、`Proxy-Authorization`
- 语义无关头：`Host`、`Content-Length`

### 响应 Header 设置

生成代码提供导出的 helper 函数，允许业务层在 handler 中设置 HTTP 响应头：

```go
func SetXxxResponseHeader(ctx context.Context, key, value string)
```

在业务实现中使用：

```go
func (s *MyService) GetItem(ctx context.Context, req *GetItemRequest) (*GetItemReply, error) {
    SetMyServiceResponseHeader(ctx, "X-Custom-Header", "value")
    // ...
}
```

## 已知限制

- 不支持 streaming RPC（client/server/bidirectional）
- `body` 具体字段仅支持顶层 message 类型（不支持标量、repeated、map）
- `body` / `response_body` 不支持 oneof 成员字段
- `response_body` 仅支持顶层字段
- 不支持复杂路径模板（如 `{name=projects/*/items/*}`），仅支持简单参数 `{id}` 和通配 `{id=*}`
- 权限选项（`public`/`auth_only`/`permission`/`permissions`/`any_permission`）互斥，不可同时设置多个

## 验证

```bash
go test ./...
go build .
go vet ./...
```

最小 proto 验证（示例）：

```bash
protoc --go-echo_out=. test.proto
```
