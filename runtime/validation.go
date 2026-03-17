package runtime

import (
	"errors"
	"reflect"
)

// DefaultValidationErrorItem 单个字段校验错误。
type DefaultValidationErrorItem struct {
	Field string `json:"field"`
	Tag   string `json:"tag"`
	Param string `json:"param"`
}

// ParseValidationErrors 从 error 中提取字段校验错误列表。
//
// 支持三种提取路径（按优先级）：
//  1. Unwrap() []error（Go 1.20+ 多错误展开）
//  2. 底层类型为 slice/array 且元素实现 fieldError 接口（如 validator/v10.ValidationErrors）
//  3. error 本身实现 fieldError 接口（单字段校验失败兜底）
func ParseValidationErrors(err error) []DefaultValidationErrorItem {
	if err == nil {
		return nil
	}

	var validationErrors []DefaultValidationErrorItem

	// fieldError 是 go-playground/validator 的字段错误接口最小子集
	type fieldError interface {
		Field() string
		Tag() string
		Param() string
	}

	// 提取单个字段错误信息的 helper
	appendFieldError := func(fe fieldError) {
		validationErrors = append(validationErrors, DefaultValidationErrorItem{
			Field: fe.Field(),
			Tag:   fe.Tag(),
			Param: fe.Param(),
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
