package runtime

import echo "github.com/labstack/echo/v4"

var binder echo.DefaultBinder

// BindPathParams 绑定 URL 路径参数。
func BindPathParams(ctx echo.Context, i any) error { return binder.BindPathParams(ctx, i) }

// BindQueryParams 绑定 URL 查询参数。
func BindQueryParams(ctx echo.Context, i any) error { return binder.BindQueryParams(ctx, i) }

// BindBody 绑定请求体。
func BindBody(ctx echo.Context, i any) error { return binder.BindBody(ctx, i) }
