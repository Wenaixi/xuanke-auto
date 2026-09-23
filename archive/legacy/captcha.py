# -*- coding: utf-8 -*-
"""
验证码识别模块：使用硅基流动（SiliconFlow）Vision 模型识别验证码图片。

配置在本地 config.py 中（SF_BASE_URL / SF_API_KEY / SF_MODEL）。

原理：
  1. 从 config.py 读取硅基流动的 base_url / api_key / 视觉模型。
  2. 调用 OpenAI 兼容的 /chat/completions 多模态接口识别验证码图片，
     返回 4 位字符。

依赖：pip install requests
"""

import base64
import json

import requests

import config

# 识别 prompt：只输出验证码字符本身
CAPTCHA_PROMPT = "请识别这张图片中的验证码字符，只输出字符本身，不要输出任何其他内容。"


def load_vision_config() -> dict:
    """从本地 config.py 加载视觉配置，返回 {"base_url", "api_key", "model"}"""
    return {
        "base_url": config.SF_BASE_URL.rstrip("/"),
        "api_key": config.SF_API_KEY,
        "model": config.SF_MODEL,
    }


def recognize_captcha(img_bytes: bytes, cfg: dict = None) -> str:
    """
    调用硅基流动 Vision 模型识别验证码图片，返回识别的字符。
    识别失败（网络/接口错误）抛出异常，由调用方决定是否换图重试。
    """
    cfg = cfg or load_vision_config()
    if not cfg.get("api_key"):
        raise RuntimeError("未配置硅基流动 API Key（config.py 的 SF_API_KEY）")

    b64 = base64.b64encode(img_bytes).decode()
    payload = {
        "model": cfg["model"],
        "messages": [{
            "role": "user",
            "content": [
                {"type": "image_url", "image_url": {"url": "data:image/jpeg;base64," + b64}},
                {"type": "text", "text": CAPTCHA_PROMPT},
            ],
        }],
        "temperature": 0,
        "max_tokens": 32,
    }
    resp = requests.post(
        cfg["base_url"] + "/chat/completions",
        headers={"Authorization": "Bearer " + cfg["api_key"], "Content-Type": "application/json"},
        json=payload,
        timeout=60,
    )
    resp.raise_for_status()
    data = resp.json()
    try:
        text = data["choices"][0]["message"]["content"].strip()
    except (KeyError, IndexError, TypeError):
        raise RuntimeError("硅基流动响应格式异常: " + json.dumps(data, ensure_ascii=False)[:200])
    return text


if __name__ == "__main__":
    # 自检：打印当前加载的视觉配置（不打印 api_key 明文）
    cfg = load_vision_config()
    print("base_url:", cfg["base_url"])
    print("model:", cfg["model"])
    print("api_key 已配置:", bool(cfg["api_key"]))
