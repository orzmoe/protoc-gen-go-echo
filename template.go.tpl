
type {{ $.InterfaceName }} interface {
{{range .InterfaceMethods}}
	{{.Name}}(context.Context, *{{.Request}}) (*{{.Reply}}, error)
{{end}}
}

type {{$.Name}}PermissionChecker interface {
	CheckPermission(ctx echo.Context, code string) error
	CheckAllPermissions(ctx echo.Context, codes []string) error
	CheckAnyPermission(ctx echo.Context, codes []string) error
	CheckAuth(ctx echo.Context) error
}

type {{$.Name}}ResponseWrapper interface {
	ParamsError(ctx echo.Context, err error) error
	Success(ctx echo.Context, data any) error
	Error(ctx echo.Context, err error) error
}

type {{$.Name}}PermissionMeta struct {
	Method      string
	Path        string
	Handler     string
	Permission  string
	IsPublic    bool
	IsAuthOnly  bool
	Summary     string
	Description string
}

func Register{{ $.InterfaceName }}(e *echo.Group, srv {{ $.InterfaceName }}, perm {{$.Name}}PermissionChecker, wrappers ...{{$.Name}}ResponseWrapper) {
	if e == nil {
		panic("Register{{$.InterfaceName}}: echo.Group must not be nil")
	}
	if srv == nil {
		panic("Register{{$.InterfaceName}}: server must not be nil")
	}
	if perm == nil {
		panic("Register{{$.InterfaceName}}: permission checker must not be nil")
	}
	resp := {{$.Name}}ResponseWrapper(default{{$.Name}}Resp{})
	if len(wrappers) > 0 && wrappers[0] != nil {
		resp = wrappers[0]
	}

	s := {{$.Name}}{
		server: srv,
		router: e,
		resp:   resp,
		perm:   perm,
		binder: new(echo.DefaultBinder),
	}
	s.RegisterService()
}

func Get{{$.Name}}PermissionMetas() []{{$.Name}}PermissionMeta {
	return []{{$.Name}}PermissionMeta{
{{range .Methods}}
		{
			Method:      {{printf "%q" .Method}},
			Path:        {{printf "%q" .Path}},
			Handler:     {{printf "%q" .HandlerName}},
			Permission:  {{printf "%q" .PermissionDisplay}},
			IsPublic:    {{.IsPublic}},
			IsAuthOnly:  {{.IsAuthOnly}},
			Summary:     {{printf "%q" .Summary}},
			Description: {{printf "%q" .Description}},
		},
{{end}}
	}
}

type {{$.Name}} struct {
	server {{ $.InterfaceName }}
	router *echo.Group
	perm   {{$.Name}}PermissionChecker
	resp   {{$.Name}}ResponseWrapper
	binder *echo.DefaultBinder
}

type default{{$.Name}}Resp struct{}

func (resp default{{$.Name}}Resp) Error(ctx echo.Context, err error) error {
	code := 500
	status := 500
	msg := "Internal Server Error"

	if err == nil {
		msg = "Unknown error, err is nil"
		return resp.buildErrorResponse(ctx, status, code, msg, err)
	}

	type iCode interface {
		HTTPCode() int
		Message() string
		Code() int
	}

	var c iCode
	if errors.As(err, &c) {
		status = c.HTTPCode()
		code = c.Code()
		msg = c.Message()
		return resp.buildErrorResponse(ctx, status, code, msg, err)
	}

	if st, ok := grpcStatus.FromError(err); ok && st.Code() != grpcCodes.OK {
		status = {{$.LowerName}}GRPCCodeToHTTPStatus(st.Code())
		code = status
		msg = st.Message()
		return resp.buildErrorResponse(ctx, status, code, msg, err)
	}

	return resp.buildErrorResponse(ctx, status, code, msg, err)
}

func (resp default{{$.Name}}Resp) buildErrorResponse(ctx echo.Context, status, code int, msg string, err error) error {
	if err != nil {
		ctx.Set("_error", err)
	}

	// 基本 3 字段 + trace_id + request_id + 开发模式字段（error_detail/stack_trace/path/method）
	responseData := make(map[string]any, 8)
	responseData["code"] = code
	responseData["msg"] = msg
	responseData["timestamp"] = time.Now().Unix()

	if traceID := get{{$.Name}}TraceID(ctx); traceID != "" {
		responseData["trace_id"] = traceID
	}
	if requestID := get{{$.Name}}RequestID(ctx); requestID != "" {
		responseData["request_id"] = requestID
	}

	if is{{$.Name}}Development() {
		if err != nil {
			responseData["error_detail"] = err.Error()
			responseData["stack_trace"] = fmt.Sprintf("%+v", err)
		}
		responseData["path"] = ctx.Request().URL.Path
		responseData["method"] = ctx.Request().Method
	}

	return ctx.JSON(status, responseData)
}

// {{$.LowerName}}IsDev 在进程启动时缓存环境判断结果，避免每次请求都调用 os.Getenv。
var {{$.LowerName}}IsDev = func() bool {
	env := os.Getenv("ENV")
	if env == "" {
		env = os.Getenv("ENVIRONMENT")
	}
	return env == "development" || env == "dev" || env == "local"
}()

func is{{$.Name}}Development() bool {
	return {{$.LowerName}}IsDev
}

func (resp default{{$.Name}}Resp) ParamsError(ctx echo.Context, err error) error {
	// 基本 3 字段 + trace_id + request_id + 详细校验字段（error_detail/validation_errors/path/method）
	responseData := make(map[string]any, 8)
	responseData["code"] = 400
	responseData["msg"] = "参数错误"
	responseData["timestamp"] = time.Now().Unix()

	if traceID := get{{$.Name}}TraceID(ctx); traceID != "" {
		responseData["trace_id"] = traceID
	}
	if requestID := get{{$.Name}}RequestID(ctx); requestID != "" {
		responseData["request_id"] = requestID
	}

	if is{{$.Name}}Development() || is{{$.Name}}DetailedValidation() {
		if err != nil {
			responseData["error_detail"] = err.Error()

			if validationErrors := parse{{$.Name}}ValidationErrors(err); len(validationErrors) > 0 {
				responseData["validation_errors"] = validationErrors
			}
		}
		responseData["path"] = ctx.Request().URL.Path
		responseData["method"] = ctx.Request().Method
	}

	return ctx.JSON(400, responseData)
}

// {{$.LowerName}}IsDetailedValidation 在进程启动时缓存，避免每次请求都调用 os.Getenv。
var {{$.LowerName}}IsDetailedValidation = os.Getenv("DETAILED_VALIDATION") == "true"

func is{{$.Name}}DetailedValidation() bool {
	return {{$.LowerName}}IsDetailedValidation
}

func parse{{$.Name}}ValidationErrors(err error) []map[string]string {
	if err == nil {
		return nil
	}

	var validationErrors []map[string]string

	type fieldError interface {
		Field() string
		Tag() string
		Param() string
	}

	// 提取单个字段错误信息的 helper
	appendFieldError := func(fe fieldError) {
		validationErrors = append(validationErrors, map[string]string{
			"field": fe.Field(),
			"tag":   fe.Tag(),
			"param": fe.Param(),
		})
	}

	// 路径 1: Unwrap() []error（Go 1.20+ 多错误展开）
	if feSlice, ok := err.(interface{ Unwrap() []error }); ok {
		for _, e := range feSlice.Unwrap() {
			if fe, ok := e.(fieldError); ok {
				appendFieldError(fe)
			}
		}
	}

	// 路径 2: error 底层类型是 slice/array，且元素实现 fieldError
	// 典型案例：go-playground/validator/v10.ValidationErrors 底层为 []FieldError，
	// 每个元素同时实现 error 和 fieldError，但整个切片不一定实现 Unwrap() []error。
	if len(validationErrors) == 0 {
		rv := reflect.ValueOf(err)
		// 如果 err 是接口/指针，先取底层值
		for rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface {
			rv = rv.Elem()
		}
		if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
			for i := range rv.Len() {
				elem := rv.Index(i).Interface()
				if fe, ok := elem.(fieldError); ok {
					appendFieldError(fe)
				}
			}
		}
	}

	// 路径 3: 错误本身就是 fieldError（单字段校验失败的兜底）
	if len(validationErrors) == 0 {
		var fe fieldError
		if errors.As(err, &fe) {
			appendFieldError(fe)
		}
	}

	return validationErrors
}

func (resp default{{$.Name}}Resp) Success(ctx echo.Context, data any) error {
	{{if $.UseWrappedResponse}}
	// 基本 3 字段 + 可选 timestamp + request_id
	responseData := make(map[string]any, 5)
	responseData["code"] = 200
	responseData["msg"] = "成功"
	// data 字段赋值（等价于 "data": data）
	responseData["data"] = data

	if is{{$.Name}}Development() && is{{$.Name}}VerboseSuccess() {
		responseData["timestamp"] = time.Now().Unix()
		if requestID := get{{$.Name}}RequestID(ctx); requestID != "" {
			responseData["request_id"] = requestID
		}
	}

	return ctx.JSON(200, responseData)
	{{else}}
	return ctx.JSON(200, data)
	{{end}}
}

// {{$.LowerName}}IsVerboseSuccess 在进程启动时缓存，避免每次请求都调用 os.Getenv。
var {{$.LowerName}}IsVerboseSuccess = os.Getenv("VERBOSE_SUCCESS") == "true"

func is{{$.Name}}VerboseSuccess() bool {
	return {{$.LowerName}}IsVerboseSuccess
}

// {{$.LowerName}}IsProduction 在进程启动时缓存生产环境判断，用于 Secure cookie 等场景。
var {{$.LowerName}}IsProduction = func() bool {
	env := os.Getenv("ENV")
	if env == "" {
		env = os.Getenv("ENVIRONMENT")
	}
	return env == "production" || env == "prod"
}()

func get{{$.Name}}TraceID(ctx echo.Context) string {
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

func get{{$.Name}}RequestID(ctx echo.Context) string {
	requestID := ctx.Response().Header().Get(echo.HeaderXRequestID)
	if requestID == "" {
		requestID = ctx.Request().Header.Get("X-Request-ID")
	}
	return requestID
}

// {{$.LowerName}}GRPCCodeToHTTPStatus 将 gRPC 状态码映射为 HTTP 状态码。
func {{$.LowerName}}GRPCCodeToHTTPStatus(c grpcCodes.Code) int {
	switch c {
	case grpcCodes.OK:
		return 200
	case grpcCodes.InvalidArgument, grpcCodes.FailedPrecondition:
		return 400
	case grpcCodes.Unauthenticated:
		return 401
	case grpcCodes.PermissionDenied:
		return 403
	case grpcCodes.NotFound:
		return 404
	case grpcCodes.AlreadyExists, grpcCodes.Aborted:
		return 409
	case grpcCodes.OutOfRange:
		return 400
	case grpcCodes.ResourceExhausted:
		return 429
	case grpcCodes.Canceled:
		return 499
	case grpcCodes.Unimplemented:
		return 501
	case grpcCodes.Unavailable:
		return 503
	case grpcCodes.DeadlineExceeded:
		return 504
	case grpcCodes.DataLoss, grpcCodes.Internal:
		return 500
	default:
		return 500
	}
}

type {{$.LowerName}}EchoCtxKey struct{}

// Get{{$.Name}}EchoContext 从 context 中获取 echo.Context（仅 upload 场景会注入值）。
func Get{{$.Name}}EchoContext(ctx context.Context) (echo.Context, bool) {
	v := ctx.Value({{$.LowerName}}EchoCtxKey{})
	if v == nil {
		return nil, false
	}
	echoCtx, ok := v.(echo.Context)
	return echoCtx, ok
}

type {{$.LowerName}}ResponseHeaderKey struct{}

// {{$.LowerName}}SkipHeaders 定义不应透传到 gRPC metadata 的 HTTP 头。
// 包含 hop-by-hop 头和敏感头，避免语义污染和信息泄露。
var {{$.LowerName}}SkipHeaders = map[string]struct{}{
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

// Set{{$.Name}}ResponseHeader 在业务层设置将要写回 HTTP 响应的 header。
func Set{{$.Name}}ResponseHeader(ctx context.Context, key, value string) {
	if headers, ok := ctx.Value({{$.LowerName}}ResponseHeaderKey{}).(map[string][]string); ok {
		headers[key] = append(headers[key], value)
	}
}

{{range .Methods}}
func (s *{{$.Name}}) {{ .HandlerName }}(ctx echo.Context) error {
	{{if not .IsPublic}}
	{{if .IsAuthOnly}}
	if err := s.perm.CheckAuth(ctx); err != nil {
		return err
	}
	{{else if .Permission}}
	if err := s.perm.CheckPermission(ctx, "{{.Permission}}"); err != nil {
		return err
	}
	{{else if .Permissions}}
	if err := s.perm.CheckAllPermissions(ctx, []string{ {{- range $index, $value := .PermissionList}}{{if $index}}, {{end}}{{printf "%q" $value}}{{end}} }); err != nil {
		return err
	}
	{{else if .AnyPermission}}
	if err := s.perm.CheckAnyPermission(ctx, []string{ {{- range $index, $value := .AnyPermissionList}}{{if $index}}, {{end}}{{printf "%q" $value}}{{end}} }); err != nil {
		return err
	}
	{{else}}
	if err := s.perm.CheckAuth(ctx); err != nil {
		return err
	}
	{{end}}
	{{end}}

	var in {{.Request}}
	{{if .IsEmptyRequest}}
	{{else}}
	{{if .UsesDefaultBinding}}
	// 默认路由继续使用 ctx.Bind，保持 path/query/body 的现有兼容行为。
	// 如果 path 参数绑定不上，请在 proto 字段上添加 @inject_tag: param:"xxx" 注解。
	if err := ctx.Bind(&in); err != nil {
		return s.resp.ParamsError(ctx, err)
	}
	{{else if .UsesPathQueryBinding}}
	// google.api.http 未指定 body 时，只绑定 path/query，不读取请求体。
	if err := s.binder.BindPathParams(ctx, &in); err != nil {
		return s.resp.ParamsError(ctx, err)
	}
	if err := s.binder.BindQueryParams(ctx, &in); err != nil {
		return s.resp.ParamsError(ctx, err)
	}
	{{else if .UsesPathBodyBinding}}
	// google.api.http body="*" 时，除路径参数外，其余字段来自请求体。
	if err := s.binder.BindPathParams(ctx, &in); err != nil {
		return s.resp.ParamsError(ctx, err)
	}
	if err := s.binder.BindBody(ctx, &in); err != nil {
		return s.resp.ParamsError(ctx, err)
	}
	{{else if .UsesPathQueryBodyFieldBinding}}
	// google.api.http body="{{.Body}}" 时，path/query 绑定到请求其他字段，body 只绑定到 in.{{.BodyFieldGoName}}。
	if err := s.binder.BindPathParams(ctx, &in); err != nil {
		return s.resp.ParamsError(ctx, err)
	}
	if err := s.binder.BindQueryParams(ctx, &in); err != nil {
		return s.resp.ParamsError(ctx, err)
	}
	if in.{{.BodyFieldGoName}} == nil {
		in.{{.BodyFieldGoName}} = new({{.BodyFieldType}})
	}
	if err := s.binder.BindBody(ctx, in.{{.BodyFieldGoName}}); err != nil {
		return s.resp.ParamsError(ctx, err)
	}
	{{end}}

	if ctx.Echo().Validator != nil {
		if err := ctx.Validate(&in); err != nil {
			return s.resp.ParamsError(ctx, err)
		}
	}
	{{end}}

	md := metadata.New(nil)
	for k, v := range ctx.Request().Header {
		if _, skip := {{$.LowerName}}SkipHeaders[k]; skip {
			continue
		}
		md.Set(k, v...)
	}
	if clientIP := ctx.RealIP(); clientIP != "" {
		if len(md.Get("x-real-ip")) == 0 {
			md.Set("x-real-ip", clientIP)
		}
	}
	if userAgent := ctx.Request().UserAgent(); userAgent != "" {
		if len(md.Get("user-agent")) == 0 {
			md.Set("user-agent", userAgent)
		}
	}

	responseHeaders := make(map[string][]string)
	newCtx := context.WithValue(ctx.Request().Context(), {{$.LowerName}}ResponseHeaderKey{}, responseHeaders)
	newCtx = metadata.NewIncomingContext(newCtx, md)
	{{if .IsUpload}}
	newCtx = context.WithValue(newCtx, {{$.LowerName}}EchoCtxKey{}, ctx)
	{{end}}

	out, err := s.server.{{.Name}}(newCtx, &in)
	if err != nil {
		return s.resp.Error(ctx, err)
	}
	if out == nil {
		return s.resp.Error(ctx, errors.New("service returned nil response without error"))
	}

	if headers, ok := newCtx.Value({{$.LowerName}}ResponseHeaderKey{}).(map[string][]string); ok {
		for key, values := range headers {
			for _, value := range values {
				ctx.Response().Header().Add(key, value)
			}
		}
	}

	{{if .IsAuthResponse}}
	authCookie := new(http.Cookie)
	authCookie.Name = "access_token"
	authCookie.Value = out.AccessToken
	authCookie.Path = "/"
	authCookie.HttpOnly = true
	authCookie.SameSite = http.SameSiteLaxMode
	// 防止 int64/uint64 -> int 溢出：钳制到合理范围
	expiresIn := int64(out.ExpiresIn)
	if expiresIn < 0 {
		expiresIn = 0
	} else if expiresIn > 315360000 { // 10 年上限
		expiresIn = 315360000
	}
	authCookie.MaxAge = int(expiresIn)
	authCookie.Secure = {{$.LowerName}}IsProduction
	ctx.SetCookie(authCookie)
	{{end}}

	{{if .IsFileDownload}}
	ctx.Response().Header().Set("Content-Type", out.ContentType)
	contentDisposition := mime.FormatMediaType("attachment", map[string]string{"filename": out.Filename})
	if contentDisposition == "" {
		// 文件名包含非法字符时回退到安全默认值
		contentDisposition = "attachment"
	}
	ctx.Response().Header().Set("Content-Disposition", contentDisposition)
	return ctx.Blob(200, out.ContentType, out.Content)
	{{else if .IsRedirect}}
	return ctx.Redirect(302, out.RedirectUrl)
	{{else if .UseResponseBody}}
	return s.resp.Success(ctx, out.{{.ResponseBodyFieldGoName}})
	{{else}}
	return s.resp.Success(ctx, out)
	{{end}}
}
{{end}}

func (s *{{$.Name}}) RegisterService() {
{{range .Methods}}
	s.router.Add("{{.Method}}", "{{.Path}}", s.{{ .HandlerName }})
{{end}}
}
