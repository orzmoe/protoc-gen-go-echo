package main

import (
	"fmt"
	"net/http"
	"strings"

	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/compiler/protogen"
)

// defaultMethod 根据函数名生成 http 路由。
// 例如: GetBlogArticles ==> GET /blog/articles
// 如果方法名首个单词不是 http method 映射，那么默认返回 POST。
func defaultMethod(file *protogen.File, m *protogen.Method, options methodOptions, imports *importManager, summary, description string) (*method, error) {
	names := strings.Split(toSnakeCase(m.GoName), "_")
	var (
		httpMethod string
		pathParts  []string
	)

	switch strings.ToUpper(names[0]) {
	case http.MethodGet, "FIND", "QUERY", "LIST", "SEARCH":
		httpMethod = http.MethodGet
		pathParts = names[1:]
	case http.MethodPost, "CREATE":
		httpMethod = http.MethodPost
		pathParts = names[1:]
	case http.MethodPut, "UPDATE":
		httpMethod = http.MethodPut
		pathParts = names[1:]
	case http.MethodPatch:
		httpMethod = http.MethodPatch
		pathParts = names[1:]
	case http.MethodDelete:
		httpMethod = http.MethodDelete
		pathParts = names[1:]
	default:
		httpMethod = "POST"
		pathParts = names
	}

	path := strings.Join(pathParts, "/")
	if path == "" {
		return nil, fmt.Errorf("%s.%s: 方法名 %q 无法推导出有效路径，请添加 google.api.http 注解或在方法名中包含资源名称", file.Desc.Path(), m.GoName, m.GoName)
	}
	md, err := buildMethodDesc(buildMethodInput{
		File:        file,
		Method:      m,
		HTTPMethod:  httpMethod,
		Path:        path,
		Body:        "*",
		BindMode:    bindModeDefault,
		Options:     options,
		Imports:     imports,
		Summary:     summary,
		Description: description,
	})
	if err != nil {
		return nil, err
	}
	return md, nil
}

// buildHTTPRule 根据 google.api.http 注解构建方法描述。
func buildHTTPRule(file *protogen.File, m *protogen.Method, rule *annotations.HttpRule, options methodOptions, imports *importManager, summary, description string) (*method, error) {
	var (
		path         string
		httpMethod   string
		bindMode     string
		requestBody  string
		responseBody string
	)
	switch pattern := rule.Pattern.(type) {
	case *annotations.HttpRule_Get:
		path = pattern.Get
		httpMethod = "GET"
	case *annotations.HttpRule_Put:
		path = pattern.Put
		httpMethod = "PUT"
	case *annotations.HttpRule_Post:
		path = pattern.Post
		httpMethod = "POST"
	case *annotations.HttpRule_Delete:
		path = pattern.Delete
		httpMethod = "DELETE"
	case *annotations.HttpRule_Patch:
		path = pattern.Patch
		httpMethod = "PATCH"
	case *annotations.HttpRule_Custom:
		path = pattern.Custom.Path
		httpMethod = strings.ToUpper(pattern.Custom.Kind)
	default:
		return nil, fmt.Errorf("%s.%s: 不支持的 google.api.http pattern", file.Desc.Path(), m.GoName)
	}
	requestBody = rule.Body
	bindMode = bindModePathQuery
	if requestBody == "*" {
		bindMode = bindModePathBody
	} else if requestBody != "" {
		bindMode = bindModePathQueryBodyField
	}
	responseBody = rule.ResponseBody
	return buildMethodDesc(buildMethodInput{
		File:         file,
		Method:       m,
		HTTPMethod:   httpMethod,
		Path:         path,
		Body:         requestBody,
		BindMode:     bindMode,
		ResponseBody: responseBody,
		Options:      options,
		Imports:      imports,
		Summary:      summary,
		Description:  description,
	})
}

// buildMethodInput 聚合 buildMethodDesc 所需的全部输入参数。
type buildMethodInput struct {
	File         *protogen.File
	Method       *protogen.Method
	HTTPMethod   string
	Path         string
	Body         string
	BindMode     string
	ResponseBody string
	Options      methodOptions
	Imports      *importManager
	Summary      string
	Description  string
}

// buildMethodDesc 根据输入参数构建模板渲染用的 method 描述。
func buildMethodDesc(in buildMethodInput) (*method, error) {
	file := in.File
	m := in.Method
	options := in.Options
	imports := in.Imports

	replyType := imports.QualifiedTypeName(m.Output)
	requestType := imports.QualifiedTypeName(m.Input)
	isAuthResponse := options.setAuthCookie

	// 响应模式：显式 option 优先，无 option 时 fallback 到字段推断
	var isFileDownload, isRedirect bool
	switch options.responseMode {
	case ResponseModeFileDownload:
		isFileDownload = true
		if err := validateFileDownloadResponse(m.Output); err != nil {
			return nil, fmt.Errorf("%s.%s: response_mode=FILE_DOWNLOAD 但 %w", file.Desc.Path(), m.GoName, err)
		}
	case ResponseModeRedirect:
		isRedirect = true
		if err := validateRedirectResponse(m.Output); err != nil {
			return nil, fmt.Errorf("%s.%s: response_mode=REDIRECT 但 %w", file.Desc.Path(), m.GoName, err)
		}
	case ResponseModeNormal:
		// 显式指定普通模式，不做推断
	default:
		// 未指定或 UNSPECIFIED，走旧字段推断
		isFileDownload = isFileDownloadResponse(m.Output)
		isRedirect = isRedirectResponse(m.Output)
	}
	normalizedPath, err := normalizeEchoPath(in.Path)
	if err != nil {
		return nil, fmt.Errorf("%s.%s: 路径 %q 不支持: %w", file.Desc.Path(), m.GoName, in.Path, err)
	}
	md := &method{
		Name:              m.GoName,
		Request:           requestType,
		Reply:             replyType,
		Path:              normalizedPath,
		Method:            in.HTTPMethod,
		Body:              in.Body,
		BindMode:          in.BindMode,
		IsEmptyRequest:    isGoogleProtobufEmpty(m.Input),
		ResponseBody:      in.ResponseBody,
		IsFileDownload:    isFileDownload,
		IsRedirect:        isRedirect,
		IsAuthResponse:    isAuthResponse,
		Permission:        options.permission,
		Permissions:       options.permissions,
		PermissionList:    splitCommaSeparated(options.permissions),
		AnyPermission:     options.anyPermission,
		AnyPermissionList: splitCommaSeparated(options.anyPermission),
		IsPublic:          options.public,
		IsAuthOnly:        options.authOnly,
		IsUpload:          options.upload,
		Summary:           in.Summary,
		Description:       in.Description,
	}

	// body 指向具体字段时，解析字段绑定信息
	if in.BindMode == bindModePathQueryBodyField {
		binding, err := resolveRequestBodyField(m.Input, in.Body, imports)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", file.Desc.Path(), m.GoName, err)
		}
		md.BodyFieldGoName = binding.GoName
		md.BodyFieldType = binding.GoType
	}

	// response_body 指向具体字段时，解析字段信息
	if in.ResponseBody != "" {
		binding, err := resolveResponseBodyField(m.Output, in.ResponseBody)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", file.Desc.Path(), m.GoName, err)
		}
		md.UseResponseBody = true
		md.ResponseBodyFieldGoName = binding.GoName
		// response_body 与特殊响应模式（下载/重定向）互斥
		if md.IsFileDownload || md.IsRedirect {
			return nil, fmt.Errorf("%s.%s: response_body 不能与文件下载或重定向同时使用", file.Desc.Path(), m.GoName)
		}
	}

	return md, nil
}

// bodyFieldBinding 描述 body 指向具体字段时的绑定信息。
type bodyFieldBinding struct {
	ProtoName string // proto 字段名，如 "user"
	GoName    string // Go 字段名，如 "User"
	GoType    string // Go 类型的 qualified name，如 "testv1.User"
	IsMessage bool   // 是否为 message 类型
}

// resolveRequestBodyField 解析 body 指向的顶层字段，返回绑定信息。
// 第一版只支持顶层 message 字段。
func resolveRequestBodyField(msg *protogen.Message, body string, imports *importManager) (*bodyFieldBinding, error) {
	for _, field := range msg.Fields {
		if string(field.Desc.Name()) == body {
			if field.Oneof != nil && !field.Oneof.Desc.IsSynthetic() {
				return nil, fmt.Errorf("body=%q 指向 oneof 成员字段，暂不支持", body)
			}
			if field.Desc.IsList() || field.Desc.IsMap() {
				return nil, fmt.Errorf("body=%q 指向 repeated/map 字段，暂不支持", body)
			}
			if field.Message == nil {
				return nil, fmt.Errorf("body=%q 指向标量字段 %s，仅支持 message 类型字段", body, field.Desc.Kind())
			}
			return &bodyFieldBinding{
				ProtoName: body,
				GoName:    field.GoName,
				GoType:    imports.QualifiedTypeName(field.Message),
				IsMessage: true,
			}, nil
		}
	}
	return nil, fmt.Errorf("body=%q 在请求消息 %s 中未找到对应字段", body, msg.Desc.Name())
}

// resolveResponseBodyField 解析 response_body 指向的顶层字段。
// 第一版只支持顶层字段（message 或 scalar 均可）。
func resolveResponseBodyField(msg *protogen.Message, fieldName string) (*bodyFieldBinding, error) {
	for _, field := range msg.Fields {
		if string(field.Desc.Name()) == fieldName {
			if field.Oneof != nil && !field.Oneof.Desc.IsSynthetic() {
				return nil, fmt.Errorf("response_body=%q 指向 oneof 成员字段，暂不支持", fieldName)
			}
			return &bodyFieldBinding{
				ProtoName: fieldName,
				GoName:    field.GoName,
				IsMessage: field.Message != nil,
			}, nil
		}
	}
	return nil, fmt.Errorf("response_body=%q 在响应消息 %s 中未找到对应字段", fieldName, msg.Desc.Name())
}
