# -*- coding: utf-8 -*-
"""
至道智慧校园登录模块（逆向自前端 JS）
实现完整登录链路：
  1. GET /login 初始化会话，取得 access_limit_cookie
  2. GET /login/captcha 获取验证码图片，会话绑定 _jfinal_captcha
  3. 账号密码 JSON 序列化后使用页面内置 RSA 公钥加密（PKCS1 v1.5）
  4. POST /login/doLogin 提交，成功后返回 token，并自动写回 config.py

验证码识别复用 Nazhi-auto 项目的硅基流动 Vision 配置（captcha.py），
识别失败会自动重试并刷新验证码。

依赖：pip install requests cryptography pyyaml
"""

import base64
import json
import time

import requests
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import padding

import captcha
import config

# 登录页内嵌的 RSA 公钥（来自 /login 页面源码，DER base64）
PUB_PEM = "MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQCWuhgriWbHIbPCQyHmablwQSyItcLyKlQU/0ydXkvU4KJtEExNmuXS0xdoVLBRGxNO5f2u2MkNGzJrFhSpVL68Qc0knhWofzs+BdtpSF4nMi7BteOvOKi0OkvhhCBcHL71Vk8UXOsaKDZkZ3lCBVQHpSA4+s6pi9xIeF93jz6pGwIDAQAB"


def _to_base36(n: int) -> str:
    """与前端 new Date().getTime().toString(36) 一致的整数转 36 进制"""
    if n == 0:
        return "0"
    digits = "0123456789abcdefghijklmnopqrstuvwxyz"
    out = ""
    while n:
        out = digits[n % 36] + out
        n //= 36
    return out


def get_unique_device_id() -> str:
    """复刻前端 getUniqueDeviceId()：UA|platform|屏幕高|屏幕宽|时间戳36进制，base64 编码"""
    parts = [
        config.HEADERS["User-Agent"],
        "Win32", "881", "1410",
        _to_base36(int(time.time() * 1000)),
    ]
    return base64.b64encode("|".join(parts).encode()).decode()


def rsa_encrypt(plain: str) -> str:
    """RSA 公钥加密（PKCS1 v1.5），输出 base64"""
    pub = serialization.load_der_public_key(base64.b64decode(PUB_PEM))
    return base64.b64encode(pub.encrypt(plain.encode(), padding.PKCS1v15())).decode()


def make_session() -> requests.Session:
    """建立带登录页头信息的会话"""
    s = requests.Session()
    s.headers.update(config.HEADERS)
    return s


def _save_token_to_config(token: str) -> None:
    """登录成功后自动把新 token 写回 config.py，避免手动复制"""
    import io

    path = "config.py"
    text = io.open(path, encoding="utf-8").read()
    text = text.replace('ID_TOKEN = "' + config.ID_TOKEN + '"', 'ID_TOKEN = "' + token + '"')
    old_cookie = config.COOKIES.get("zd_edu_cookie", "")
    text = text.replace(
        '"zd_edu_cookie": "' + old_cookie + '"',
        '"zd_edu_cookie": "' + token + '"')
    io.open(path, "w", encoding="utf-8", newline="").write(text)
    print("[登录] 已更新 config.py 的 ID_TOKEN / zd_edu_cookie")


def login(account: str, password: str, max_retry: int = 10, vision_config: dict = None):
    """
    完整登录，返回 (已登录会话, token)。
    验证码识别走硅基流动 Vision（captcha.py 加载 Nazhi-auto 配置），失败自动重试。
    """
    vision_config = vision_config or captcha.load_vision_config()

    for attempt in range(1, max_retry + 1):
        s = make_session()
        s.get(config.BASE_URL + "/login", timeout=10)
        img = s.get(config.BASE_URL + "/login/captcha?v=" + str(time.time()), timeout=10).content
        try:
            captcha_text = captcha.recognize_captcha(img, vision_config)
        except Exception as e:
            print("[登录] 第" + str(attempt) + "次验证码识别失败: " + str(e) + "，重试")
            time.sleep(0.8)
            continue

        payload = {
            "captcha": captcha_text,
            "identification": rsa_encrypt(json.dumps({
                "userName": account,
                "password": password,
            }, ensure_ascii=False)),
            "uniqueId": get_unique_device_id(),
            "priorityId": "",
        }
        resp = s.post(config.BASE_URL + "/login/doLogin", data=payload, timeout=10)
        data = resp.json()
        if data.get("isOk"):
            token = data.get("token")
            s.cookies.set("zd_edu_cookie", token)
            _save_token_to_config(token)
            return s, token
        print("[登录] 第" + str(attempt) + "次失败: " + str(data.get("msg")) + "，刷新验证码重试")
        time.sleep(0.8)

    raise RuntimeError("登录失败：验证码识别 " + str(max_retry) + " 次均未通过")


if __name__ == "__main__":
    session, token = login(config.ACCOUNT, config.PASSWORD)
    print("登录成功，token:", token)
    print("cookies:", session.cookies.get_dict())
