package runtime

import "os"

// 进程级环境变量缓存，在 init 阶段求值一次，避免每次请求调用 os.Getenv。

var isDev = func() bool {
	env := os.Getenv("ENV")
	if env == "" {
		env = os.Getenv("ENVIRONMENT")
	}
	return env == "development" || env == "dev" || env == "local"
}()

var isProduction = func() bool {
	env := os.Getenv("ENV")
	if env == "" {
		env = os.Getenv("ENVIRONMENT")
	}
	return env == "production" || env == "prod"
}()

var isDetailedValidation = os.Getenv("DETAILED_VALIDATION") == "true"

var isVerboseSuccess = os.Getenv("VERBOSE_SUCCESS") == "true"

// IsDevelopment 返回当前进程是否运行在开发模式。
// 判定条件：ENV 或 ENVIRONMENT 为 development / dev / local。
func IsDevelopment() bool {
	return isDev
}

// IsProduction 返回当前进程是否运行在生产模式。
// 判定条件：ENV 或 ENVIRONMENT 为 production / prod。
func IsProduction() bool {
	return isProduction
}

// IsDetailedValidation 返回是否启用详细校验错误输出。
// 判定条件：DETAILED_VALIDATION=true。
func IsDetailedValidation() bool {
	return isDetailedValidation
}

// IsVerboseSuccess 返回是否启用成功响应的额外信息（时间戳、request_id）。
// 判定条件：VERBOSE_SUCCESS=true。
func IsVerboseSuccess() bool {
	return isVerboseSuccess
}
