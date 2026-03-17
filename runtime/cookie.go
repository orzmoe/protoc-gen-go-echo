package runtime

import (
	"net/http"

	echo "github.com/labstack/echo/v4"
)

// SetAuthCookie 设置 access_token cookie。
// expiresIn 为过期时间（秒），会被钳制到 [0, 315360000]（最多 10 年）。
// Secure 标志根据 [IsProduction] 自动判定。
func SetAuthCookie(ctx echo.Context, token string, expiresIn int64) {
	if expiresIn < 0 {
		expiresIn = 0
	} else if expiresIn > 315360000 { // 10 年上限
		expiresIn = 315360000
	}
	ctx.SetCookie(&http.Cookie{
		Name:     "access_token",
		Value:    token,
		Path:     "/",
		MaxAge:   int(expiresIn),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   IsProduction(),
	})
}

// ClearAuthCookie 清除 access_token cookie（用于登出场景）。
func ClearAuthCookie(ctx echo.Context) {
	ctx.SetCookie(&http.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   IsProduction(),
	})
}
