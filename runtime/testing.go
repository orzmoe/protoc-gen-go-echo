package runtime

// SetEnvOverrides 覆盖环境检测标志，用于测试场景。
// 调用返回的函数可恢复原始值。
//
// 示例:
//
//	restore := runtime.SetEnvOverrides(true, false, false, false)
//	defer restore()
func SetEnvOverrides(development, production, detailedValidation, verboseSuccess bool) (restore func()) {
	old := [4]bool{isDev, isProduction, isDetailedValidation, isVerboseSuccess}
	isDev = development
	isProduction = production
	isDetailedValidation = detailedValidation
	isVerboseSuccess = verboseSuccess
	return func() {
		isDev, isProduction, isDetailedValidation, isVerboseSuccess = old[0], old[1], old[2], old[3]
	}
}
