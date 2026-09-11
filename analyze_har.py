# -*- coding: utf-8 -*-
import json
import os

har_files = ['www.zhidao.fj.cn.har', '课程www.zhidao.fj.cn.har']

def analyze_har(file_path):
    if not os.path.exists(file_path):
        print(f"File not found: {file_path}")
        return
    print(f"\n=================== ANALYZING: {file_path} ===================")
    with open(file_path, 'r', encoding='utf-8', errors='ignore') as f:
        data = json.load(f)

    entries = data.get('log', {}).get('entries', [])
    print(f"Total entries: {len(entries)}")

    elective_entries = []
    for e in entries:
        req = e.get('request', {})
        url = req.get('url', '')
        if 'electives' in url or 'login' in url:
            method = req.get('method')
            status = e.get('response', {}).get('status')
            post_data = req.get('postData', {})
            post_text = post_data.get('text', '')
            post_mime = post_data.get('mimeType', '')

            headers = {h['name']: h['value'] for h in req.get('headers', [])}
            cookies = {c['name']: c['value'] for c in req.get('cookies', [])}

            resp_content = e.get('response', {}).get('content', {})
            resp_text = resp_content.get('text', '')
            if len(resp_text) > 300:
                resp_preview = resp_text[:300] + '...'
            else:
                resp_preview = resp_text

            print(f"\n[REQUEST] {method} {url}")
            print(f"Status: {status}")
            print(f"Content-Type: {headers.get('Content-Type', headers.get('content-type', 'N/A'))}")
            print(f"Cookie Keys: {list(cookies.keys())}")
            if 'access_limit_cookie' in cookies:
                print(f"access_limit_cookie: {cookies['access_limit_cookie']}")
            if 'zd_edu_cookie' in cookies:
                print(f"zd_edu_cookie: {cookies['zd_edu_cookie']}")
            print(f"Post mime: {post_mime}, Post data: {post_text}")
            print(f"Response preview: {resp_preview}")

for h in har_files:
    analyze_har(h)
