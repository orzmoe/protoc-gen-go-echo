// Package runtime 提供 protoc-gen-go-echo 生成代码的共享运行时基础设施。
//
// 生成的 Echo HTTP handler 代码依赖此包中的接口定义、默认响应实现、
// 环境判断、gRPC 状态码映射、请求上下文管理等能力，
// 避免每个 service 重复生成相同的基础设施代码。
package runtime

import (
	echo "github.com/labstack/echo/v4"
)

// ResponseWrapper 定义 HTTP 响应的统一包装接口。
// 生成代码中的默认实现为 [DefaultWrappedResp] 和 [DefaultDirectResp]，
// 用户可通过 Register 函数注入自定义实现来覆盖默认行为。
type ResponseWrapper interface {
	// ParamsError 处理参数校验失败的响应。
	ParamsError(ctx echo.Context, err error) error
	// Success 处理成功响应。
	Success(ctx echo.Context, data any) error
	// Error 处理业务/系统错误响应。
	Error(ctx echo.Context, err error) error
}

// PermissionChecker 定义权限校验接口。
// 生成的 handler 在执行业务逻辑前通过此接口检查请求权限。
type PermissionChecker interface {
	// CheckPermission 检查单个权限标识。
	CheckPermission(ctx echo.Context, code string) error
	// CheckAllPermissions 检查所有权限（AND 语义）。
	CheckAllPermissions(ctx echo.Context, codes []string) error
	// CheckAnyPermission 检查任意权限（OR 语义）。
	CheckAnyPermission(ctx echo.Context, codes []string) error
	// CheckAuth 仅检查认证状态，不检查具体权限。
	CheckAuth(ctx echo.Context) error
}

// PermissionMeta 描述单个路由的权限元信息，用于权限管理后台同步。
type PermissionMeta struct {
	Method         string
	Path           string
	Handler        string
	Permission     string
	Permissions    []string
	PermissionMode string
	IsPublic       bool
	IsAuthOnly     bool
	Summary        string
	Description    string
}
