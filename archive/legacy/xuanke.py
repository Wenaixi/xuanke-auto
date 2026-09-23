# -*- coding: utf-8 -*-
"""
知道教育平台 自动选课脚本
逆向结果：
  学期列表  POST /electives/select?idToken=..                      学期列表（先调用）
  课程数据  POST /electives/select/findElectivesData               课程列表（body: schoolYear/schoolTerm）
  实时人数  POST /electives/select/findElectivesStudentCount        body: ids=1,2,3（逗号分隔）
  班级详情  POST /electives/classDetail?idToken=..                  body: id=<课程id>（上课地点/授课老师/课节）
  报名      POST /electives/select/selectElectivesClass             body: classId=<课程id>
  退选      POST /electives/select/exitElectivesClass               body: classId=<课程id>
注意：提交选课默认关闭，需将 AUTO_SUBMIT 改为 True 并确认目标课程后再用。
"""

import time
import json
from datetime import datetime

import requests

import config


# 是否开启自动提交选课（false=只查询不提交，改为 True 才会实际报名）
AUTO_SUBMIT = False

# 目标课程：支持课程 id 或课程名（子串匹配，可用于批量盯同一门课的不同班次）
# 示例：TARGETS = ["健美操"] 或 TARGETS = [61245]
TARGETS: list = []


def make_session() -> requests.Session:
    s = requests.Session()
    s.headers.update(config.HEADERS)
    s.cookies.update(config.COOKIES)
    return s


def api_url(path: str) -> str:
    return config.BASE_URL + path + "?idToken=" + config.ID_TOKEN


def fetch_year_terms(session: requests.Session) -> list:
    """获取可选学年学期列表"""
    resp = session.post(api_url("/electives/select"), timeout=10)
    resp.raise_for_status()
    data = resp.json()
    if not data.get("isOk"):
        raise RuntimeError("接口返回错误: " + str(data.get("msg", "未知错误")))
    return data.get("currentYearTermList", [])


def fetch_electives(session: requests.Session, school_year=None, school_term=None) -> dict:
    """查询当前所有选课发布数据。
    根据 HAR 抓包与真实 JS 分析：直接 POST 空请求体，服务端会自动采用当前激活学期并返回全量课程。
    """
    if school_year is None or school_term is None:
        try:
            # 优先采用真实浏览器原生行为：直接 POST 空 body
            resp = session.post(api_url("/electives/select/findElectivesData"), timeout=10)
            resp.raise_for_status()
            data = resp.json()
            if data.get("isOk") and data.get("selectElectivesData"):
                return data
        except Exception:
            pass

        # 备选回退：尝试从学期列表获取选中学期
        for term in fetch_year_terms(session):
            if term.get("selected"):
                school_year, school_term = term.get("schoolYear"), term.get("schoolTerm")
                break
        else:
            raise RuntimeError("未找到当前学期")

    resp = session.post(
        api_url("/electives/select/findElectivesData"),
        json={"schoolYear": school_year, "schoolTerm": school_term},
        timeout=10,
    )
    resp.raise_for_status()
    data = resp.json()
    if not data.get("isOk"):
        raise RuntimeError("接口返回错误: " + str(data.get("msg", "未知错误")))
    return data


def fetch_class_detail(session: requests.Session, class_id: int) -> dict:
    """获取课程/班级详情（点击课程名弹出的内容：上课地点、授课老师、课节等）"""
    resp = session.post(
        api_url("/electives/classDetail"),
        data={"id": class_id},
        timeout=10,
    )
    resp.raise_for_status()
    data = resp.json()
    if not data.get("isOk"):
        raise RuntimeError("接口返回错误: " + str(data.get("msg", "未知错误")))
    return data.get("value", {})


def match_class(cls: dict, targets: list) -> bool:
    """判断课程是否命中目标：TARGETS 为空则全部命中（需可报名）"""
    if not targets:
        return True
    for t in targets:
        if isinstance(t, int) and cls.get("id") == t:
            return True
        if isinstance(t, str) and t in (cls.get("course_name") or ""):
            return True
    return False


def print_electives(data: dict) -> None:
    """格式化打印课程列表"""
    sep = "=" * 60
    begin_times = data.get("beginTimes", [])

    print()
    print(sep)
    print("查询时间: " + datetime.now().strftime("%Y-%m-%d %H:%M:%S"))
    for ts in begin_times:
        dt = datetime.fromtimestamp(ts / 1000)
        print("开放时间: " + dt.strftime("%Y-%m-%d %H:%M:%S"))
    print(sep)

    for publish in data.get("selectElectivesData", []):
        print()
        print("[发布] " + str(publish.get("publishName")) +
              "  (可选:" + str(publish.get("canSelect")) +
              "  已选:" + str(publish.get("hasSelected")) + ")")
        for cls in publish.get("electivesClassList", []):
            status = "可报名" if cls.get("can_select") else "不可报名"
            print("  id=%-6d  %-16s  %-10s  %3d/%3d人  %s  %s" % (
                cls["id"],
                cls.get("course_name", ""),
                cls.get("teacher_name_list", ""),
                cls.get("selected_count", 0),
                cls.get("max_count", 0),
                status,
                cls.get("class_room_name", ""),
            ))


def print_class_detail(value: dict) -> None:
    """格式化打印课程详情（弹窗内容）"""
    print()
    print("=" * 60)
    print("课程详情")
    print("=" * 60)
    print("课程名称: " + str(value.get("course_name", "")))
    print("选修班:   " + str(value.get("class_name", "")))
    print("授课老师: " + str(value.get("teacher_name", "")))
    print("上课地点: " + str(value.get("classroom_name", "")))
    print("上课课节: " + str(value.get("lessons_date", "")))
    print("学年学期: " + str(value.get("school_year_term", "")))
    print("课程类型: " + str(value.get("course_type_name", "")))
    print("任课方式: " + str(value.get("method_name", "")))
    print("评价方式: " + str(value.get("evaluate_type_name", "")))
    print("班级人数: " + str(value.get("audited_count", 0)) + "/" + str(value.get("plan_count", 0)))
    print("课程状态: " + str(value.get("class_status_str", "")))
    print("分享链接: " + str(value.get("shareUrl", "")))


def find_target_classes(data: dict) -> list:
    """找出命中的可报名课程"""
    result = []
    for publish in data.get("selectElectivesData", []):
        for cls in publish.get("electivesClassList", []):
            if not cls.get("can_select"):
                continue
            if match_class(cls, TARGETS):
                result.append(cls)
    return result


def submit_elective(session: requests.Session, cls: dict) -> dict:
    """提交选课报名（classId 字段已抓包验证）"""
    resp = session.post(
        api_url("/electives/select/selectElectivesClass"),
        data={"classId": cls["id"]},
        timeout=10,
    )
    resp.raise_for_status()
    return resp.json()


def query_mode(session: requests.Session) -> None:
    """仅查询，打印课程信息后退出"""
    data = fetch_electives(session)
    print_electives(data)


def detail_mode(session: requests.Session, class_ids: list) -> None:
    """查询指定课程 id 的详情（上课地点/授课老师等）"""
    if not class_ids:
        print("用法: python xuanke.py detail 61245 61115")
        return
    for cid in class_ids:
        try:
            value = fetch_class_detail(session, cid)
            print_class_detail(value)
        except RuntimeError as e:
            print("[接口错误] " + str(e))


def monitor_mode(session: requests.Session) -> None:
    """
    监控模式：每隔 POLL_INTERVAL 秒轮询一次。
    当 inDateRange 变为 True 且 AUTO_SUBMIT=True 时，自动提交目标课程。
    """
    print("进入监控模式，每 " + str(config.POLL_INTERVAL) + " 秒查询一次，Ctrl+C 退出")
    submitted = set()

    while True:
        try:
            data = fetch_electives(session)
            print_electives(data)

            for publish in data.get("selectElectivesData", []):
                if AUTO_SUBMIT and publish.get("inDateRange"):
                    targets = [c for c in publish.get("electivesClassList", [])
                               if c.get("can_select") and match_class(c, TARGETS)]
                    for cls in targets:
                        if cls["id"] in submitted:
                            continue
                        print("\n>>> 正在提交: " + str(cls.get("course_name")) + " (id=" + str(cls["id"]) + ")")
                        result = submit_elective(session, cls)
                        print("    结果: " + str(result))
                        if result.get("isOk"):
                            submitted.add(cls["id"])
                            print("    报名成功！")
                        else:
                            print("    报名失败: " + str(result.get("msg")))

        except requests.RequestException as e:
            print("[网络错误] " + str(e) + "，" + str(config.POLL_INTERVAL) + " 秒后重试")
        except RuntimeError as e:
            print("[接口错误] " + str(e))
            if "Cookie" in str(e) or "登录" in str(e):
                print("Cookie 可能已过期，请运行 python login.py 重新登录并更新 config.py")
                break

        time.sleep(config.POLL_INTERVAL)


if __name__ == "__main__":
    import sys

    session = make_session()
    args = sys.argv[1:]
    mode = args[0] if args else "query"

    if mode == "monitor":
        monitor_mode(session)
    elif mode == "detail":
        detail_mode(session, [int(a) for a in args[1:]])
    else:
        query_mode(session)
