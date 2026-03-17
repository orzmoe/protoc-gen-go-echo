package runtime

// SetEnvOverrides 覆盖环境检测标志，用于测试场景。
// 调用返回的函数可恢复原始值。
//
// 示例:
//
//	restore := runtime.SetEnvOverrides(true, false)
//	defer restore()
func SetEnvOverrides(development, detailedValidation bool) (restore func()) {
	oldDev, oldDetailed := isDev, isDetailedValidation
	isDev = development
	isDetailedValidation = detailedValidation
	return func() {
		isDev = oldDev
		isDetailedValidation = oldDetailed
	}
}
