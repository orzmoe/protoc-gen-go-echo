package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed template.go.tpl
var tpl string

var echoTemplate = template.Must(template.New("echo").Parse(strings.TrimSpace(tpl)))

type service struct {
	Name          string // Greeter
	FullName      string // helloworld.Greeter
	FilePath      string // api/helloworld/helloworld.proto
	ResponseStyle string // wrapped|direct

	Methods          []*method
	InterfaceMethods []*method
}

func (s *service) execute() (string, error) {
	buf := new(bytes.Buffer)
	if err := echoTemplate.Execute(buf, s); err != nil {
		return "", fmt.Errorf("execute echo template for %s: %w", s.Name, err)
	}
	return buf.String(), nil
}

// InterfaceName service interface name
func (s *service) InterfaceName() string {
	return s.Name + "HTTPServer"
}

// UseWrappedResponse 判断是否使用包装响应格式。
func (s *service) UseWrappedResponse() bool {
	return s.ResponseStyle != responseStyleDirect
}

// LowerName 返回首字母小写的服务名称，用于生成 unexported 标识符。
func (s *service) LowerName() string {
	if s.Name == "" {
		return ""
	}
	return strings.ToLower(s.Name[:1]) + s.Name[1:]
}

type method struct {
	// ── 路由标识 ──
	Name   string // RPC 方法名，如 "GetItem"
	Num    int    // 同名方法的序号（additional_bindings 场景）
	Method string // HTTP 方法，如 "GET"
	Path   string // Echo 路由路径，如 "/v1/items/:id"

	// ── 请求/响应类型 ──
	Request        string // 请求类型的 qualified name，如 "testv1.GetItemReq"
	Reply          string // 响应类型的 qualified name，如 "testv1.GetItemReply"
	IsEmptyRequest bool   // 请求是否为 google.protobuf.Empty

	// ── 请求绑定 ──
	Body            string // google.api.http body 值：""/"*"/"<field>"
	BindMode        string // 绑定模式：default/path_query/path_body/path_query_body_field
	BodyFieldGoName string // body 指向具体字段时的 Go 字段名，如 "User"
	BodyFieldType   string // body 指向具体字段时的 Go 类型，如 "testv1.User"

	// ── 响应模式 ──
	ResponseBody            string // google.api.http response_body 值
	UseResponseBody         bool   // 是否使用 response_body 选择子字段
	ResponseBodyFieldGoName string // response_body 对应的 Go 字段名，如 "Value"
	IsFileDownload          bool   // 是否为文件下载响应
	IsRedirect              bool   // 是否为重定向响应
	IsAuthResponse          bool   // 是否设置 auth cookie

	// ── 权限 ──
	Permission        string   // 单权限标识
	Permissions       string   // 多权限（逗号分隔，AND 语义）
	PermissionList    []string // Permissions 拆分后的列表
	AnyPermission     string   // 任意权限（逗号分隔，OR 语义）
	AnyPermissionList []string // AnyPermission 拆分后的列表
	IsPublic          bool     // 是否公开（无需认证）
	IsAuthOnly        bool     // 仅需认证（不检查具体权限）

	// ── 元信息 ──
	IsUpload    bool   // 是否为上传场景（注入 echo.Context 到 context）
	Summary     string // @summary 注释内容
	Description string // @description 注释内容
}

// HandlerName 返回 Echo handler 的去重名称（Name_Num）。
func (m *method) HandlerName() string {
	return fmt.Sprintf("%s_%d", m.Name, m.Num)
}

func (m *method) UsesDefaultBinding() bool {
	return m.BindMode == bindModeDefault
}

func (m *method) UsesPathQueryBinding() bool {
	return m.BindMode == bindModePathQuery
}

func (m *method) UsesPathBodyBinding() bool {
	return m.BindMode == bindModePathBody
}

func (m *method) UsesPathQueryBodyFieldBinding() bool {
	return m.BindMode == bindModePathQueryBodyField
}

// PermissionDisplay 返回用于元信息展示的权限标识。
// Permission / Permissions / AnyPermission 三者互斥，取第一个非空值。
func (m *method) PermissionDisplay() string {
	switch {
	case m.Permission != "":
		return m.Permission
	case m.Permissions != "":
		return m.Permissions
	case m.AnyPermission != "":
		return m.AnyPermission
	default:
		return ""
	}
}
