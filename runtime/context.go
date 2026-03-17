package runtime

import (
	"context"

	echo "github.com/labstack/echo/v4"
	"google.golang.org/grpc/metadata"
)

// requestState 聚合每个请求的上下文状态（unexported，不暴露给用户）。
type requestState struct {
	responseHeaders map[string][]string
	echoCtx         echo.Context // 仅 upload 场景赋值
}

// requestStateKey 是 requestState 在 context.Context 中的 key 类型。
type requestStateKey struct{}

// skipHeaders 定义不应透传到 gRPC metadata 的 HTTP 头。
// 包含 hop-by-hop 头和敏感头，避免语义污染和信息泄露。
var skipHeaders = map[string]struct{}{
	"Connection":          {},
	"Keep-Alive":          {},
	"Proxy-Authenticate":  {},
	"Proxy-Authorization": {},
	"Te":                  {},
	"Trailer":             {},
	"Transfer-Encoding":   {},
	"Upgrade":             {},
	"Host":                {},
	"Content-Length":      {},
}

// BuildIncomingContext 将 HTTP 请求头转发为 gRPC metadata，
// 并在返回的 context 中挂载请求状态（用于响应 header 累积和 upload 场景的 echo.Context 传递）。
//
// isUpload 为 true 时，会将 echo.Context 存入请求状态，
// 业务层可通过 [GetEchoContext] 获取。
func BuildIncomingContext(ctx echo.Context, isUpload bool) context.Context {
	// 防重入：如果 context 中已有 state，复用而非覆盖
	reqCtx := ctx.Request().Context()
	if _, ok := reqCtx.Value(requestStateKey{}).(*requestState); ok {
		return reqCtx
	}

	req := ctx.Request()
	hdr := req.Header
	md := make(metadata.MD, len(hdr)+2)
	var hasXRealIP, hasUserAgent bool
	for k, v := range hdr {
		if _, skip := skipHeaders[k]; skip {
			continue
		}
		switch k {
		case "X-Real-Ip":
			hasXRealIP = true
		case "User-Agent":
			hasUserAgent = true
		}
		md.Set(k, v...)
	}
	if !hasXRealIP {
		if clientIP := ctx.RealIP(); clientIP != "" {
			md.Set("x-real-ip", clientIP)
		}
	}
	if !hasUserAgent {
		if ua := req.UserAgent(); ua != "" {
			md.Set("user-agent", ua)
		}
	}

	state := &requestState{
		responseHeaders: make(map[string][]string),
	}
	if isUpload {
		state.echoCtx = ctx
	}

	newCtx := context.WithValue(reqCtx, requestStateKey{}, state)
	newCtx = metadata.NewIncomingContext(newCtx, md)
	return newCtx
}

// WriteResponseHeaders 将请求状态中累积的响应 header 写回 HTTP 响应。
// reqCtx 应为 [BuildIncomingContext] 返回的 context。
func WriteResponseHeaders(echoCtx echo.Context, reqCtx context.Context) {
	state, ok := reqCtx.Value(requestStateKey{}).(*requestState)
	if !ok || len(state.responseHeaders) == 0 {
		return
	}
	for key, values := range state.responseHeaders {
		for _, value := range values {
			echoCtx.Response().Header().Add(key, value)
		}
	}
}

// SetResponseHeader 在业务层设置将要写回 HTTP 响应的 header。
// ctx 应为经过 [BuildIncomingContext] 处理后的 context。
//
// 注意：此函数不是并发安全的，不应在多个 goroutine 中同时调用。
func SetResponseHeader(ctx context.Context, key, value string) {
	if state, ok := ctx.Value(requestStateKey{}).(*requestState); ok {
		state.responseHeaders[key] = append(state.responseHeaders[key], value)
	}
}

// GetEchoContext 从 context 中获取 echo.Context。
// 仅在 upload 场景下（[BuildIncomingContext] 的 isUpload 为 true 时）会返回有效值。
func GetEchoContext(ctx context.Context) (echo.Context, bool) {
	if state, ok := ctx.Value(requestStateKey{}).(*requestState); ok && state.echoCtx != nil {
		return state.echoCtx, true
	}
	return nil, false
}
