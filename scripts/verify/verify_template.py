#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
tm-stock 联调验证脚本标准模板

用法：
    1. 复制为 verify_{业务}.py（如 verify_theme_query.py）
    2. 改 BASE_URL / 账号
    3. 在「验证场景」区用 check() 写断言
    4. python scripts/verify/verify_xxx.py

设计说明（沉淀自踩坑经验）：
    - 禁止在 bash 里内嵌 python/curl 管道做联调——引号地狱 + 无法复用 + 出错难查
    - 统一 check() 断言，末尾汇总 ✅/❌，退出码非 0 便于接 CI
    - token 自动获取与续期，避免每个脚本各写一套
    - 真实密钥不写在脚本里：从环境变量读（见 CLAUDE.md#凭据约定）
"""

import os
import sys
import json
import urllib.request
import urllib.error

# ── 配置（真实密钥只从环境变量读，禁止硬编码）──
BASE_URL = os.getenv("TM_API_BASE", "http://127.0.0.1:8080")
USERNAME = os.getenv("TM_TEST_USER", "")
PASSWORD = os.getenv("TM_TEST_PASSWORD", "")

_token = None
_results = []


def _req(method, path, body=None, auth=True):
    """统一请求：自动带 token、自动解析 JSON。"""
    url = path if path.startswith("http") else BASE_URL + path
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    if auth and _token:
        req.add_header("Authorization", "Bearer " + _token)
    try:
        with urllib.request.urlopen(req, timeout=15) as r:
            raw = r.read().decode("utf-8")
            return r.status, (json.loads(raw) if raw else {})
    except urllib.error.HTTPError as e:
        raw = e.read().decode("utf-8", "ignore")
        try:
            return e.code, json.loads(raw)
        except Exception:
            return e.code, {"raw": raw}
    except Exception as e:
        return 0, {"error": str(e)}


def login():
    """获取 token；失败不中断，让后续断言如实报错。"""
    global _token
    if not USERNAME:
        print("[提示] 未设置 TM_TEST_USER，跳过登录（仅验证免鉴权接口）")
        return
    code, body = _req("POST", "/api/v1/auth/login",
                      {"username": USERNAME, "password": PASSWORD}, auth=False)
    if code == 200 and body.get("code") == 0:
        _token = (body.get("data") or {}).get("token")
        print("[登录成功]")
    else:
        print("[登录失败] code=%s body=%s" % (code, body))


def check(name, cond, detail=""):
    """统一断言：只记录，不中断，最后汇总。"""
    _results.append((name, bool(cond), detail))
    print(("  ✅ " if cond else "  ❌ ") + name + (("  | " + str(detail)) if detail else ""))
    return bool(cond)


def summary():
    ok = sum(1 for _, c, _ in _results if c)
    total = len(_results)
    print("")
    print("=" * 46)
    print("  验证结果：%d / %d 通过" % (ok, total))
    if ok != total:
        print("  失败项：")
        for n, c, d in _results:
            if not c:
                print("    - %s %s" % (n, d))
    print("=" * 46)
    return 0 if ok == total else 1


# ══════════════════ 验证场景（按需修改）══════════════════
def scenario_health():
    print("\n[场景] 服务健康检查")
    code, body = _req("GET", "/healthz", auth=False)
    check("healthz 返回 200", code == 200, "实际 %s" % code)
    check("响应 code=0", body.get("code") == 0, body)


def scenario_example_theme():
    """示例：题材查询（接口实现后按实际契约改）"""
    print("\n[场景] 题材查询")
    code, body = _req("GET", "/api/v1/themes?page=1&size=10")
    if not check("接口可达", code == 200, "实际 %s" % code):
        return
    data = body.get("data") or {}
    check("返回 list 字段", isinstance(data.get("list"), list))
    check("空数据返回 [] 而非 null", data.get("list") is not None)
    # 合规断言：一期不得返回个股字段
    first = (data.get("list") or [{}])[0]
    forbidden = [k for k in ("stocks", "components", "tsCode", "stockList") if k in first]
    check("未返回个股字段（合规红线）", not forbidden, forbidden)


def main():
    print("目标环境：%s" % BASE_URL)
    login()
    scenario_health()
    # scenario_example_theme()   # 接口就绪后打开
    sys.exit(summary())


if __name__ == "__main__":
    main()
