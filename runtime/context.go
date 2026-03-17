package runtime

import (
	"context"
	"net/http"
	"strings"
	"sync"

	echo "github.com/labstack/echo/v4"
	"google.golang.org/grpc/metadata"
)

// requestState 聚合每个请求的上下文状态（unexported，不暴露给用户）。
type requestState struct {
	mu              sync.Mutex
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
	"X-Real-Ip":           {},
}

// BuildIncomingContext 将 HTTP 请求头转发为 gRPC metadata，
// 并在返回的 context 中挂载请求状态（用于响应 header 累积和 upload 场景的 echo.Context 传递）。
//
// isUpload 为 true 时，会将 echo.Context 存入请求状态，
// 业务层可通过 [GetEchoContext] 获取。
func BuildIncomingContext(ctx echo.Context, isUpload bool) context.Context {
	// 防重入：如果 context 中已有 state，复用而非覆盖
	reqCtx := ctx.Request().Context()
	if existing, ok := reqCtx.Value(requestStateKey{}).(*requestState); ok {
		// 补写 echoCtx：首次非 upload 调用后，后续 upload 调用需要补充
		if isUpload {
			existing.mu.Lock()
			if existing.echoCtx == nil {
				existing.echoCtx = ctx
			}
			existing.mu.Unlock()
		}
		return reqCtx
	}

	req := ctx.Request()
	hdr := req.Header
	md := make(metadata.MD, len(hdr)+2)
	var hasUserAgent bool
	dynamicSkip := make(map[string]struct{})
	if connValues := hdr.Values("Connection"); len(connValues) > 0 {
		for _, connValue := range connValues {
			for _, token := range strings.Split(connValue, ",") {
				token = strings.TrimSpace(token)
				if token == "" {
					continue
				}
				dynamicSkip[http.CanonicalHeaderKey(token)] = struct{}{}
			}
		}
	}
	for k, v := range hdr {
		canonicalKey := http.CanonicalHeaderKey(k)
		if _, skip := skipHeaders[canonicalKey]; skip {
			continue
		}
		if _, skip := dynamicSkip[canonicalKey]; skip {
			continue
		}
		switch canonicalKey {
		case "User-Agent":
			hasUserAgent = true
		}
		md.Set(k, v...)
	}
	if clientIP := ctx.RealIP(); clientIP != "" {
		md.Set("x-real-ip", clientIP)
	}
	if !hasUserAgent {
		if ua := req.UserAgent(); ua != "" {
			md.Set("user-agent", ua)
		}
	}

	if existingMD, ok := metadata.FromIncomingContext(reqCtx); ok {
		for k, v := range existingMD {
			if _, exists := md[k]; !exists {
				md[k] = v
			}
		}
	}

	state := &requestState{}
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
	if !ok {
		return
	}
	state.mu.Lock()
	if len(state.responseHeaders) == 0 {
		state.mu.Unlock()
		return
	}
	headers := make(map[string][]string, len(state.responseHeaders))
	for key, values := range state.responseHeaders {
		headers[key] = append([]string(nil), values...)
	}
	state.mu.Unlock()
	for key, values := range headers {
		for _, value := range values {
			echoCtx.Response().Header().Add(key, value)
		}
	}
}

// SetResponseHeader 在业务层设置将要写回 HTTP 响应的 header。
// ctx 应为经过 [BuildIncomingContext] 处理后的 context。
func SetResponseHeader(ctx context.Context, key, value string) {
	if state, ok := ctx.Value(requestStateKey{}).(*requestState); ok {
		state.mu.Lock()
		if state.responseHeaders == nil {
			state.responseHeaders = make(map[string][]string)
		}
		state.responseHeaders[key] = append(state.responseHeaders[key], value)
		state.mu.Unlock()
	}
}

// GetEchoContext 从 context 中获取 echo.Context。
// 仅在 upload 场景下（[BuildIncomingContext] 的 isUpload 为 true 时）会返回有效值。
func GetEchoContext(ctx context.Context) (echo.Context, bool) {
	if state, ok := ctx.Value(requestStateKey{}).(*requestState); ok {
		state.mu.Lock()
		echoCtx := state.echoCtx
		state.mu.Unlock()
		if echoCtx != nil {
			return echoCtx, true
		}
	}
	return nil, false
}
