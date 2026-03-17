
type {{ $.InterfaceName }} interface {
{{range .InterfaceMethods}}
	{{.Name}}(context.Context, *{{.Request}}) (*{{.Reply}}, error)
{{end}}
}

func Register{{ $.InterfaceName }}(e *echo.Group, srv {{ $.InterfaceName }}, perm runtime.PermissionChecker, wrappers ...runtime.ResponseWrapper) {
	if e == nil {
		panic("Register{{$.InterfaceName}}: echo.Group must not be nil")
	}
	if srv == nil {
		panic("Register{{$.InterfaceName}}: server must not be nil")
	}
	if perm == nil {
		panic("Register{{$.InterfaceName}}: permission checker must not be nil")
	}
	{{if $.UseWrappedResponse}}
	resp := runtime.ResponseWrapper(runtime.DefaultWrappedResp{})
	{{else}}
	resp := runtime.ResponseWrapper(runtime.DefaultDirectResp{})
	{{end}}
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

func Get{{$.Name}}PermissionMetas() []runtime.PermissionMeta {
	return []runtime.PermissionMeta{
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

// Set{{$.Name}}ResponseHeader 在业务层设置将要写回 HTTP 响应的 header。
// 注意：此函数不是并发安全的，不应在多个 goroutine 中同时调用。
func Set{{$.Name}}ResponseHeader(ctx context.Context, key, value string) {
	runtime.SetResponseHeader(ctx, key, value)
}

// Get{{$.Name}}EchoContext 从 context 中获取 echo.Context（仅 upload 场景会注入值）。
func Get{{$.Name}}EchoContext(ctx context.Context) (echo.Context, bool) {
	return runtime.GetEchoContext(ctx)
}

type {{$.Name}} struct {
	server {{ $.InterfaceName }}
	router *echo.Group
	perm   runtime.PermissionChecker
	resp   runtime.ResponseWrapper
	binder *echo.DefaultBinder
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

	newCtx := runtime.BuildIncomingContext(ctx, {{.IsUpload}})

	out, err := s.server.{{.Name}}(newCtx, &in)
	if err != nil {
		return s.resp.Error(ctx, err)
	}
	if out == nil {
		return s.resp.Error(ctx, errors.New("service returned nil response without error"))
	}

	runtime.WriteResponseHeaders(ctx, newCtx)

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
	authCookie.Secure = runtime.IsProduction()
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
