package main

import (
	"log"

	"xuanke-auto/backend/internal/zhidao"
)

func main() {
	c := zhidao.New("https://www.zhidao.fj.cn", zhidao.VisionConfig{})
	c.SetCredentials("", "", "***REMOVED***")
	c.SetCookies(map[string]string{
		"menu_sidebar_scroll_top": "434.4801025390625",
		"access_limit_cookie":     "***REMOVED***",
		"zd_edu_cookie":           "***REMOVED***",
	})
	msg, err := c.SelectClass(-1)
	if err != nil {
		log.Printf("报名接口已连通（预期报错）: %v", err)
	} else {
		log.Printf("意外成功: %s", msg)
	}
}
