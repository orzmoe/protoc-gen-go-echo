
type {{ $.InterfaceName }} interface {
{{range .InterfaceMethods}}
	{{.Name}}(context.Context, *{{.Request}}) (*{{.Reply}}, error)
{{end}}
}

// Register{{$.InterfaceName}} 注册 HTTP 路由。
// wrappers 为可选参数，最多接受一个自定义 [runtime.ResponseWrapper]，超过一个将被忽略。
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
	}
	s.RegisterService()
}

var {{$.LowerName}}PermissionMetas = []runtime.PermissionMeta{
{{range .Methods}}
	{
		Method:      {{printf "%q" .Method}},
		Path:        {{printf "%q" .Path}},
		Handler:     {{printf "%q" .HandlerName}},
		Permission:  {{printf "%q" .PermissionDisplay}},
		{{if .IsPublic}}
		PermissionMode: "public",
		{{else if .IsAuthOnly}}
		PermissionMode: "auth_only",
		{{else if .Permission}}
		PermissionMode: "single",
		Permissions:    []string{ {{printf "%q" .Permission}} },
		{{else if .Permissions}}
		PermissionMode: "all",
		Permissions:    []string{ {{range $i, $v := .PermissionList}}{{if $i}}, {{end}}{{printf "%q" $v}}{{end}} },
		{{else if .AnyPermission}}
		PermissionMode: "any",
		Permissions:    []string{ {{range $i, $v := .AnyPermissionList}}{{if $i}}, {{end}}{{printf "%q" $v}}{{end}} },
		{{else}}
		PermissionMode: "auth_only",
		{{end}}
		IsPublic:    {{.IsPublic}},
		IsAuthOnly:  {{.IsAuthOnly}},
		Summary:     {{printf "%q" .Summary}},
		Description: {{printf "%q" .Description}},
	},
{{end}}
}

func Get{{$.Name}}PermissionMetas() []runtime.PermissionMeta {
	src := {{$.LowerName}}PermissionMetas
	dst := make([]runtime.PermissionMeta, len(src))
	copy(dst, src)
	for i := range dst {
		if dst[i].Permissions != nil {
			dst[i].Permissions = slices.Clone(dst[i].Permissions)
		}
	}
	return dst
}

type {{$.Name}} struct {
	server {{ $.InterfaceName }}
	router *echo.Group
	perm   runtime.PermissionChecker
	resp   runtime.ResponseWrapper
}

{{range .Methods}}
func (s *{{$.Name}}) {{ .HandlerName }}(ctx echo.Context) error {
	{{if not .IsPublic}}
	{{if .IsAuthOnly}}
	if err := s.perm.CheckAuth(ctx); err != nil {
		return s.resp.Error(ctx, err)
	}
	{{else if .Permission}}
	if err := s.perm.CheckPermission(ctx, "{{.Permission}}"); err != nil {
		return s.resp.Error(ctx, err)
	}
	{{else if .Permissions}}
	if err := s.perm.CheckAllPermissions(ctx, []string{ {{- range $index, $value := .PermissionList}}{{if $index}}, {{end}}{{printf "%q" $value}}{{end}} }); err != nil {
		return s.resp.Error(ctx, err)
	}
	{{else if .AnyPermission}}
	if err := s.perm.CheckAnyPermission(ctx, []string{ {{- range $index, $value := .AnyPermissionList}}{{if $index}}, {{end}}{{printf "%q" $value}}{{end}} }); err != nil {
		return s.resp.Error(ctx, err)
	}
	{{else}}
	if err := s.perm.CheckAuth(ctx); err != nil {
		return s.resp.Error(ctx, err)
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
	if err := runtime.BindPathParams(ctx, &in); err != nil {
		return s.resp.ParamsError(ctx, err)
	}
	if err := runtime.BindQueryParams(ctx, &in); err != nil {
		return s.resp.ParamsError(ctx, err)
	}
	{{else if .UsesPathBodyBinding}}
	// google.api.http body="*" 时，除路径参数外，其余字段来自请求体。
	if err := runtime.BindPathParams(ctx, &in); err != nil {
		return s.resp.ParamsError(ctx, err)
	}
	if err := runtime.BindBody(ctx, &in); err != nil {
		return s.resp.ParamsError(ctx, err)
	}
	{{else if .UsesPathQueryBodyFieldBinding}}
	// google.api.http body="{{.Body}}" 时，path/query 绑定到请求其他字段，body 只绑定到 in.{{.BodyFieldGoName}}。
	if err := runtime.BindPathParams(ctx, &in); err != nil {
		return s.resp.ParamsError(ctx, err)
	}
	if err := runtime.BindQueryParams(ctx, &in); err != nil {
		return s.resp.ParamsError(ctx, err)
	}
	if in.{{.BodyFieldGoName}} == nil {
		in.{{.BodyFieldGoName}} = new({{.BodyFieldType}})
	}
	if err := runtime.BindBody(ctx, in.{{.BodyFieldGoName}}); err != nil {
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
	runtime.WriteResponseHeaders(ctx, newCtx)
	if err != nil {
		return s.resp.Error(ctx, err)
	}
	if out == nil {
		return s.resp.Error(ctx, errors.New("service returned nil response without error"))
	}

	{{if .IsAuthResponse}}
	runtime.SetAuthCookie(ctx, out.AccessToken, int64(out.ExpiresIn))
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
