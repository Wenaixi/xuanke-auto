# 知道教育平台选课自动化配置
# Cookie 失效后可运行 python login.py 重新登录（自动识别验证码）

BASE_URL = "https://www.zhidao.fj.cn"

# idToken 即 zd_edu_cookie 的值，所有 API 请求都需要作为 URL 参数携带
ID_TOKEN = "***REMOVED***"

# 登录账号（python login.py 可自动获取新 token 并提示更新本文件）
ACCOUNT = "***REMOVED***"
PASSWORD = "***REMOVED***"

# 验证码识别（硅基流动 Vision 模型）
SF_BASE_URL = "https://api.siliconflow.cn/v1"
SF_API_KEY = "***REMOVED***"
SF_MODEL = "Qwen/Qwen3-VL-30B-A3B-Instruct"

COOKIES = {
    "access_limit_cookie": "***REMOVED***",
    "zd_edu_cookie": "***REMOVED***",
}

HEADERS = {
    "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.0.0 Safari/537.36",
    "X-Requested-With": "XMLHttpRequest",
    "Accept": "application/json, text/javascript, */*; q=0.01",
    "Referer": "https://www.zhidao.fj.cn/admin.html",
    "Origin": "https://www.zhidao.fj.cn",
}

# 轮询间隔（秒），等待选课窗口开启时使用
POLL_INTERVAL = 5
