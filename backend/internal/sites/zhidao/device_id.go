package zhidao

import (
	"encoding/base64"
	"strconv"
	"strings"
	"time"
)

// uniqueDeviceID 复刻站点前端 getUniqueDeviceId()：
// base64(UA|platform|屏幕高|屏幕宽|毫秒时间戳转36进制)。
// 屏幕尺寸为固定值（站点只校验格式，不校验具体值）。
func uniqueDeviceID(ua string, now time.Time) string {
	parts := []string{ua, "Win32", "881", "1410", strconv.FormatInt(now.UnixMilli(), 36)}
	return base64.StdEncoding.EncodeToString([]byte(strings.Join(parts, "|")))
}
