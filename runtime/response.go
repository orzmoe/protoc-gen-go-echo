package runtime

import (
	"errors"
	"fmt"
	"time"

	echo "github.com/labstack/echo/v4"
	grpcCodes "google.golang.org/grpc/codes"
	grpcStatus "google.golang.org/grpc/status"
)

// ── 响应结构体 ──

// DefaultErrorResponse 错误响应的固定结构。
type DefaultErrorResponse struct {
	Code         int    `json:"code"`
	Msg          string `json:"msg"`
	Timestamp    int64  `json:"timestamp"`
	TraceID      string `json:"trace_id,omitempty"`
	RequestID    string `json:"request_id,omitempty"`
	ErrorDetail  string `json:"error_detail,omitempty"`
	ErrorVerbose string `json:"error_verbose,omitempty"`
	Path         string `json:"path,omitempty"`
	Method       string `json:"method,omitempty"`
}

// DefaultParamsErrorResponse 参数错误响应的固定结构。
type DefaultParamsErrorResponse struct {
	Code             int                          `json:"code"`
	Msg              string                       `json:"msg"`
	Timestamp        int64                        `json:"timestamp"`
	TraceID          string                       `json:"trace_id,omitempty"`
	RequestID        string                       `json:"request_id,omitempty"`
	ErrorDetail      string                       `json:"error_detail,omitempty"`
	ValidationErrors []DefaultValidationErrorItem `json:"validation_errors,omitempty"`
	Path             string                       `json:"path,omitempty"`
	Method           string                       `json:"method,omitempty"`
}

// DefaultSuccessResponse wrapped 模式的成功响应结构。
type DefaultSuccessResponse struct {
	Code      int    `json:"code"`
	Msg       string `json:"msg"`
	Data      any    `json:"data"`
	Timestamp int64  `json:"timestamp,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// ── Observability helpers ──

// GetTraceID 从 HTTP 请求中提取 trace ID。
// 优先读取 X-Trace-ID 请求头，其次读取 echo context 中的 trace_id 值。
func GetTraceID(ctx echo.Context) string {
	if traceID := ctx.Request().Header.Get("X-Trace-ID"); traceID != "" {
		return traceID
	}
	if traceID := ctx.Get("trace_id"); traceID != nil {
		if id, ok := traceID.(string); ok {
			return id
		}
	}
	return ""
}

// GetRequestID 从 HTTP 响应或请求头中提取 request ID。
func GetRequestID(ctx echo.Context) string {
	requestID := ctx.Response().Header().Get(echo.HeaderXRequestID)
	if requestID == "" {
		requestID = ctx.Request().Header.Get("X-Request-ID")
	}
	return requestID
}

// ── DefaultWrappedResp: wrapped 模式 {code,msg,data} ──

// DefaultWrappedResp 实现 [ResponseWrapper]，使用 {code, msg, data} 包装格式。
type DefaultWrappedResp struct{}

func (resp DefaultWrappedResp) Error(ctx echo.Context, err error) error {
	code := 500
	status := 500
	msg := "Internal Server Error"

	if err == nil {
		msg = "Unknown error, err is nil"
		return resp.buildErrorResponse(ctx, status, code, msg, err)
	}

	type iCode interface {
		error
		HTTPCode() int
		Message() string
		Code() int
	}

	if c, ok := errors.AsType[iCode](err); ok {
		status = c.HTTPCode()
		code = c.Code()
		msg = c.Message()
		return resp.buildErrorResponse(ctx, status, code, msg, err)
	}

	if st, ok := grpcStatus.FromError(err); ok && st.Code() != grpcCodes.OK {
		status = GRPCCodeToHTTPStatus(st.Code())
		code = status
		msg = st.Message()
		return resp.buildErrorResponse(ctx, status, code, msg, err)
	}

	return resp.buildErrorResponse(ctx, status, code, msg, err)
}

func (DefaultWrappedResp) buildErrorResponse(ctx echo.Context, status, code int, msg string, err error) error {
	if err != nil {
		ctx.Set("_error", err)
	}

	r := DefaultErrorResponse{
		Code:      code,
		Msg:       msg,
		Timestamp: time.Now().Unix(),
		TraceID:   GetTraceID(ctx),
		RequestID: GetRequestID(ctx),
	}

	if IsDevelopment() {
		if err != nil {
			r.ErrorDetail = err.Error()
			r.ErrorVerbose = fmt.Sprintf("%+v", err)
		}
		r.Path = ctx.Request().URL.Path
		r.Method = ctx.Request().Method
	}

	return ctx.JSON(status, &r)
}

func (resp DefaultWrappedResp) ParamsError(ctx echo.Context, err error) error {
	r := DefaultParamsErrorResponse{
		Code:      400,
		Msg:       "参数错误",
		Timestamp: time.Now().Unix(),
		TraceID:   GetTraceID(ctx),
		RequestID: GetRequestID(ctx),
	}

	if IsDevelopment() || IsDetailedValidation() {
		if err != nil {
			r.ErrorDetail = err.Error()
			r.ValidationErrors = ParseValidationErrors(err)
		}
	}
	if IsDevelopment() {
		r.Path = ctx.Request().URL.Path
		r.Method = ctx.Request().Method
	}

	return ctx.JSON(400, &r)
}

func (DefaultWrappedResp) Success(ctx echo.Context, data any) error {
	r := DefaultSuccessResponse{
		Code: 200,
		Msg:  "成功",
		Data: data,
	}

	if IsDevelopment() && IsVerboseSuccess() {
		r.Timestamp = time.Now().Unix()
		r.RequestID = GetRequestID(ctx)
	}

	return ctx.JSON(200, &r)
}

// ── DefaultDirectResp: direct 模式 ──

// DefaultDirectResp 实现 [ResponseWrapper]，成功响应直接返回数据，不包装。
// Error 和 ParamsError 行为与 [DefaultWrappedResp] 一致。
type DefaultDirectResp struct{}

func (resp DefaultDirectResp) Error(ctx echo.Context, err error) error {
	return DefaultWrappedResp{}.Error(ctx, err)
}

func (resp DefaultDirectResp) ParamsError(ctx echo.Context, err error) error {
	return DefaultWrappedResp{}.ParamsError(ctx, err)
}

func (DefaultDirectResp) Success(ctx echo.Context, data any) error {
	return ctx.JSON(200, data)
}
