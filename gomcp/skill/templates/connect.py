"""把游戏服内嵌的 MCP 调试服务(gomcp)注册进本机 Claude Code。

用法:
    connect.py connect <地址> [--token T] [--name N] [--scope local|project]
    connect.py list
    connect.py remove <名字> [--project]
    connect.py tools <地址>                    列出该服务器的工具(不注册)
    connect.py call <地址> <工具> [--args JSON] 直接调用一个工具(不注册)

connect/list/remove 走 Claude Code 的 MCP 注册,工具要重启会话才进来;
tools/call 直接打 HTTP 端点,不注册也不用重启,适合临时看一眼别的服务器。

地址接受 127.0.0.1:8300 / http://192.168.0.21:8300 / 192.168.0.21:8300 三种写法。

作用域:
    local(默认)  写本机 ~/.claude.json 本项目段,只对自己生效,token 不入库
    project      写项目根 .mcp.json,提交后全团队共用——适合测试机这种大家都要连的地址;
                 该文件要入库,所以拒绝同时传 --token

.mcp.json 是入库的团队共享配置,凡涉及它的写入 / 删除都必须显式带参数:
新增要 `connect --scope project`,删除要 `remove --project`。
不带参数的命令只动本机,绝不碰这个文件。
"""

from __future__ import annotations

import argparse
import json
import re
import shutil
import subprocess
import sys
import urllib.error
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]

# Windows 上 Python 默认按 locale(GBK)编码输出,本脚本的中文提示会变乱码。
# 强制 UTF-8,让 cmd / PowerShell / Git Bash 下的输出一致可读。
for _s in (sys.stdout, sys.stderr):
    if hasattr(_s, "reconfigure"):
        _s.reconfigure(encoding="utf-8")

# 注册名统一前缀,便于 list/remove 只管本 skill 建的条目,不误伤其它 MCP
# 由 yyds/gomcp/cmd/skill 生成时按 appid 填入
PREFIX = "__APPID__-"
LOCALHOST = {"127.0.0.1", "localhost", "::1", "[::1]", "0.0.0.0"}
TIMEOUT = 12


def _normalize(addr: str) -> tuple[str, str, str]:
    """把用户输入的地址补全成 (url, host, port)。"""
    a = addr.strip()
    if not a:
        raise ValueError("地址不能为空")
    if not re.match(r"^https?://", a):
        a = "http://" + a
    m = re.match(r"^(https?)://([^/:]+|\[[^\]]+\])(?::(\d+))?", a)
    if not m:
        raise ValueError(f"无法解析地址:{addr}")
    scheme, host, port = m.group(1), m.group(2), m.group(3)
    if not port:
        raise ValueError(f"地址缺少端口:{addr}(gomcp 默认端口 8300,例:127.0.0.1:8300)")
    return f"{scheme}://{host}:{port}", host, port


def _check_name(name: str) -> None:
    """claude mcp add 只接受字母/数字/连字符/下划线,中文等一律拒绝。

    提前挡在这里,而不是等跑完 initialize 握手再由 CLI 报错——那样白等一次网络往返,
    错误信息也是英文的。
    """
    if not re.fullmatch(r"[A-Za-z0-9_-]+", name):
        raise ValueError(
            f"注册名不合法:{name}\n"
            "  Claude Code 只接受字母、数字、连字符、下划线(不支持中文)。\n"
            "  例:--name __APPID__-local、--name __APPID__-test")


def _derive_name(host: str, port: str) -> str:
    """本地地址只带端口更好认;非本地带上 host 以便区分是哪台机器。"""
    if host.lower() in LOCALHOST:
        return f"{PREFIX}{port}"
    slug = re.sub(r"[^0-9a-zA-Z]+", "-", host).strip("-").lower()
    return f"{PREFIX}{slug}-{port}"


def _claude(args: list[str]) -> tuple[int, str]:
    """调用 claude CLI。

    Windows 上 claude 是 claude.CMD,subprocess 不走 PATHEXT 解析,
    直接传 "claude" 会 FileNotFoundError,必须先用 shutil.which 拿全路径。
    """
    exe = shutil.which("claude")
    if not exe:
        return 127, ("未找到 claude CLI:确认已安装 Claude Code 且 claude 在 PATH 中"
                     "(npm 全局安装时通常在 %APPDATA%\\npm)")
    try:
        p = subprocess.run([exe, *args], capture_output=True, text=True,
                           encoding="utf-8", errors="replace", check=False)
    except OSError as e:
        return 127, f"调用 claude 失败:{e}"
    return p.returncode, (p.stdout or "") + (p.stderr or "")


def _rpc(url: str, token: str, method: str, params: dict | None = None,
         timeout: int = TIMEOUT) -> tuple[bool, object]:
    """向 MCP 端点发一次 JSON-RPC,返回 (成功?, result 或错误说明)。

    gomcp 跑在 Stateless 模式,不需要先 initialize 建会话,可以直接 tools/call。
    响应是 SSE(`data: {...}`),这里统一解析成 JSON。
    """
    body: dict = {"jsonrpc": "2.0", "id": 1, "method": method}
    if params is not None:
        body["params"] = params
    headers = {
        "Content-Type": "application/json",
        "Accept": "application/json, text/event-stream",
    }
    if token:
        headers["Authorization"] = f"Bearer {token}"
    req = urllib.request.Request(url, data=json.dumps(body).encode("utf-8"),
                                 method="POST", headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            raw = r.read().decode("utf-8", "replace")
    except urllib.error.HTTPError as e:
        if e.code == 401:
            return False, ("HTTP 401:目标服配了 [mcp].token,需要用 --token 传同一个值"
                           "(值在对方的 config.toml 里)")
        return False, f"HTTP {e.code}"
    except urllib.error.URLError as e:
        return False, _explain(str(e.reason))
    except Exception as e:  # noqa: BLE001
        return False, _explain(str(e))

    for line in raw.splitlines():
        s = line[5:].strip() if line.startswith("data:") else line.strip()
        if not s.startswith("{"):
            continue
        try:
            d = json.loads(s)
        except ValueError:
            continue
        if "error" in d:
            err = d["error"]
            return False, f"服务端返回错误:{err.get('message', err)}"
        if "result" in d:
            return True, d["result"]
    return False, f"响应里没有 result(片段:{raw[:150]})"


def _verify(url: str, token: str) -> tuple[bool, str]:
    """走一次 MCP initialize 握手验连通。失败时按类型给可操作的提示。"""
    ok, r = _rpc(url, token, "initialize", {
        "protocolVersion": "2025-06-18", "capabilities": {},
        "clientInfo": {"name": "gomcp", "version": "1.0"},
    })
    return (True, "initialize OK") if ok else (False, str(r))


def _explain(reason: str) -> str:
    low = reason.lower()
    if "refused" in low or "拒绝" in reason:
        return ("连接被拒:目标服没启动,或它的 config.toml 没配 [mcp].address。"
                "\n  远程连不上最常见的原因是对方绑了 127.0.0.1(gomcp 默认值,只监听本机),"
                "需要对方改成 0.0.0.0 或具体网卡 IP 后重启。")
    if "timed out" in low or "timeout" in low:
        return "连接超时:检查地址是否可达、有没有防火墙挡住该端口。"
    if "closed connection" in low or "empty reply" in low or "badstatusline" in low:
        #TCP 通但没有 HTTP 响应,说明这个端口上根本不是 HTTP 服务
        return ("端口能连上,但对方不是 HTTP/MCP 服务(没有任何 HTTP 响应就断开)。常见原因:"
                "\n  - 该端口被别的服务占用,换一个 [mcp].address 端口"
                "\n  - 目标服的版本还没有 gomcp(r3640 起才有),需要部署新版 server.exe"
                "\n  - 目标服 config.toml 没配 [mcp].address,没启动 MCP 服务")
    return f"连接失败:{reason}"


def _shared_names() -> set[str]:
    """读 .mcp.json 拿到项目级(入库共享)的条目名,用于给 list 标注来源。

    直接读文件而不是逐个 claude mcp get,省掉 N 次子进程调用。
    """
    p = ROOT / ".mcp.json"
    if not p.exists():
        return set()
    try:
        return set((json.loads(p.read_text(encoding="utf-8")) or {}).get("mcpServers", {}))
    except (OSError, ValueError):
        return set()


def _entries() -> list[tuple[str, str]]:
    """从 claude mcp list 里挑出本 skill 建的条目,返回 (名字, 地址)。

    list 的行格式是 `<name>: <url> - ✔ Connected`,名字和地址一次拿全,
    不用再逐个 claude mcp get。
    """
    rc, out = _claude(["mcp", "list"])
    if rc != 0:
        return []
    r = []
    for line in out.splitlines():
        m = re.match(rf"\s*({re.escape(PREFIX)}[0-9a-zA-Z-]+):\s*(\S+)", line)
        if m:
            r.append((m.group(1), m.group(2)))
    return r


def cmd_connect(args: argparse.Namespace) -> int:
    try:
        url, host, port = _normalize(args.address)
        name = args.name or _derive_name(host, port)
        _check_name(name)
    except ValueError as e:
        print(f"error: {e}", file=sys.stderr)
        return 2
    token = (args.token or "").strip()
    scope = args.scope
    if scope == "project" and token:
        # .mcp.json 要入版本库,写进去的 token 等于公开。宁可拦住,也不留一个
        # 「提交后才发现秘密进了库」的坑。
        print("error: --scope project 会把配置写进入库的 .mcp.json,token 将明文公开。\n"
              "  要么让目标服不配 [mcp].token(内网调试服的推荐做法),\n"
              "  要么用 --scope local 各自注册(token 只留在本机 ~/.claude.json)。",
              file=sys.stderr)
        return 2
    if scope == "local" and name in _shared_names():
        # 同名条目分处两个作用域会造成困惑,但清掉项目级那条等于替全团队改环境。
        # 挡在校验之前:这条路怎么都走不通,没必要先白跑一次网络往返。
        print(f"error: {name} 已是 .mcp.json 里的项目级条目(入版本库、全团队共用)。\n"
              "  这里不替你删团队配置。\n"
              f"  换个名字注册到本机:--name {name}-local\n"
              "  或确实要覆盖团队配置:显式加 --scope project(改动需提交 .mcp.json)。",
              file=sys.stderr)
        return 2

    # 先验后注册:注册一个连不上的条目只会在 /mcp 里多一个红叉,不如当场报错指路
    print(f"校验 {url} ...")
    ok, msg = _verify(url, token)
    if not ok:
        print(f"error: {msg}", file=sys.stderr)
        print("未注册任何条目。", file=sys.stderr)
        return 1
    print(f"  {msg}")

    #幂等:只清目标作用域。曾经这里两个作用域一起清,于是一句默认 local 的 connect
    #就能把入库 .mcp.json 里的同名团队条目顺手删掉——涉及项目级的动作一律要显式带参数。
    _claude(["mcp", "remove", "--scope", scope, name])
    cmd = ["mcp", "add", "--scope", scope, "--transport", "http", name, url]
    if token:
        cmd += ["--header", f"Authorization: Bearer {token}"]
    rc, out = _claude(cmd)
    if rc != 0:
        print(f"error: 注册失败:{out.strip()}", file=sys.stderr)
        return 1
    if scope == "project":
        print(f"已注册 {name} -> {url} (project 作用域,写入 .mcp.json)")
        print("  .mcp.json 需要提交到版本库,团队成员 update 后即可用,不必各自注册。")
        print("  他们首次会看到该 server 处于 Pending approval,批准一次即可。")
    else:
        print(f"已注册 {name} -> {url} (local 作用域,只对你生效,token 不入库)")
    #Claude Code 在会话启动时就确定了 MCP server 列表,中途新增的不会出现在 /mcp 面板,
    #那里的 Reconnect 只能重连已在列表中的条目,对新注册的无效。
    print(f"\n下一步:重启 Claude Code 会话(退出重进,或 IDE 里 Reload Window)。"
          f"\n新注册的 MCP server 要重启才会进入会话——/mcp 面板里的 Reconnect 只对"
          f"已在列表中的条目有效,找不到 {name} 是正常的。"
          f"\n重启后工具名形如 mcp__{name}__server_status。")
    return 0


def cmd_list(_args: argparse.Namespace) -> int:
    entries = _entries()
    if not entries:
        print("没有已注册的 gomcp 条目。用 connect <地址> 添加。")
        return 0
    shared = _shared_names()
    for name, url in entries:
        #不带 token 复验:配了 token 的条目会回 401,如实标注即可
        ok, msg = _verify(url, "")
        state = "连通" if ok else f"不通 - {msg.splitlines()[0]}"
        origin = "项目共享" if name in shared else "本机"
        print(f"{name}: {url}  [{state}] [{origin}]")
    return 0


def cmd_remove(args: argparse.Namespace) -> int:
    #默认只动 local。删项目级条目会改入库的 .mcp.json,等于替全团队改环境,
    #必须显式 --project——「移除一个我自己加的调试地址」和「删掉团队共享配置」
    #是两件事,不该由同一句不带参数的命令同时完成。
    if args.name in _shared_names() and not args.project:
        print(f"error: {args.name} 是 .mcp.json 里的项目级条目(入版本库、全团队共用)。\n"
              "  确实要删就加 --project,删完记得提交 .mcp.json。", file=sys.stderr)
        return 2

    scopes = ("local", "project") if args.project else ("local",)
    ok = False
    last = ""
    for s in scopes:
        rc, out = _claude(["mcp", "remove", "--scope", s, args.name])
        if rc == 0:
            ok = True
        else:
            last = out.strip()
    if not ok:
        print(f"error: 移除失败:{last}", file=sys.stderr)
        return 1
    if args.project:
        _prune_mcp_json()
        print(f"已移除 {args.name}(含项目级条目,记得提交 .mcp.json)")
    else:
        print(f"已移除 {args.name}")
    return 0


def cmd_tools(args: argparse.Namespace) -> int:
    """列出目标服务器提供的工具。不注册、不进 MCP 列表。"""
    try:
        url, _, _ = _normalize(args.address)
    except ValueError as e:
        print(f"error: {e}", file=sys.stderr)
        return 2
    ok, r = _rpc(url, (args.token or "").strip(), "tools/list")
    if not ok:
        print(f"error: {r}", file=sys.stderr)
        return 1
    tools = (r or {}).get("tools") or []
    print(f"{url} 提供 {len(tools)} 个工具:\n")
    for t in tools:
        print(f"  {t.get('name')}")
        if desc := t.get("description"):
            print(f"      {desc}")
    return 0


def cmd_call(args: argparse.Namespace) -> int:
    """直接调用目标服务器的一个工具。

    走原始 HTTP,不经 Claude Code 的 MCP 注册——不用注册、不用重启会话,
    也不会在 /mcp 列表里留下条目。适合临时看一眼别的服务器。
    """
    try:
        url, _, _ = _normalize(args.address)
    except ValueError as e:
        print(f"error: {e}", file=sys.stderr)
        return 2
    try:
        arguments = json.loads(args.args) if args.args else {}
    except ValueError as e:
        print(f"error: --args 不是合法 JSON:{e}", file=sys.stderr)
        return 2
    if not isinstance(arguments, dict):
        print("error: --args 必须是 JSON 对象,如 '{\"uid\":\"21h02rw\"}'", file=sys.stderr)
        return 2

    #工具里有 pprof_capture 这种要跑十几秒的,超时放宽
    ok, r = _rpc(url, (args.token or "").strip(), "tools/call",
                 {"name": args.tool, "arguments": arguments}, timeout=args.timeout)
    if not ok:
        print(f"error: {r}", file=sys.stderr)
        return 1
    r = r or {}
    for c in r.get("content") or []:
        if c.get("type") == "text":
            print(c.get("text", ""))
        else:
            print(json.dumps(c, ensure_ascii=False, indent=2))
    if r.get("isError"):
        return 1  #工具自身报错(如参数不对),让调用方能从退出码看出来
    return 0


def _prune_mcp_json() -> None:
    """.mcp.json 里最后一个条目被移除后,claude 会留下 {"mcpServers": {}} 空壳。
    留着只会让版本库里多一个没有内容的文件,直接删掉。
    """
    p = ROOT / ".mcp.json"
    if not p.exists():
        return
    try:
        d = json.loads(p.read_text(encoding="utf-8")) or {}
    except (OSError, ValueError):
        return
    if not d.get("mcpServers") and set(d) <= {"mcpServers"}:
        p.unlink(missing_ok=True)


def main() -> int:
    p = argparse.ArgumentParser(description="把游戏服 MCP 调试服务注册进 Claude Code")
    sub = p.add_subparsers(dest="command", required=True)

    c = sub.add_parser("connect", help="校验并注册一个 gomcp 地址")
    c.add_argument("address", help="地址,如 127.0.0.1:8300 或 http://192.168.0.21:8300")
    c.add_argument("--token", help="目标服 [mcp].token 的值(对方配了才需要)")
    c.add_argument("--name", help="注册名,默认从地址推导")
    c.add_argument("--scope", choices=("local", "project"), default="local",
                   help="local=只对自己生效(默认);project=写入库的 .mcp.json,全团队共享")
    c.set_defaults(func=cmd_connect)

    sub.add_parser("list", help="列出已注册的 gomcp 条目及连通状态").set_defaults(func=cmd_list)

    r = sub.add_parser("remove", help="移除一个已注册条目(默认只动 local)")
    r.add_argument("name")
    r.add_argument("--project", action="store_true",
                   help="连项目级 .mcp.json 里的条目一起删(会改入库文件,删完需提交)")
    r.set_defaults(func=cmd_remove)

    t = sub.add_parser("tools", help="列出目标服务器的工具(不注册)")
    t.add_argument("address")
    t.add_argument("--token")
    t.set_defaults(func=cmd_tools)

    cl = sub.add_parser("call", help="直接调用目标服务器的工具(不注册、不用重启会话)")
    cl.add_argument("address", help="地址,如 192.168.0.21:8300")
    cl.add_argument("tool", help="工具名,如 server_status;用 tools 子命令可列出")
    cl.add_argument("--args", help="工具入参,JSON 对象字符串,如 '{\"uid\":\"21h02rw\"}'")
    cl.add_argument("--token")
    cl.add_argument("--timeout", type=int, default=60,
                    help="超时秒数,默认 60(pprof_capture 这类耗时工具可调大)")
    cl.set_defaults(func=cmd_call)

    args = p.parse_args()
    return args.func(args)


if __name__ == "__main__":
    sys.exit(main())
