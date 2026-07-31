---
name: gomcp
description: 连接游戏服内嵌的 MCP 调试服务（gomcp）：把地址注册进 Claude Code，或不注册直接调用某台服务器的调试工具（查状态 / 查配置 / GM 指令 / pprof）。当用户说 "/gomcp <地址:端口>"、"连游戏服 mcp"、"注册 mcp 服务器"、"接入 gomcp"、"连调试服务"、"把 mcp 加进来"、"换个服务器连"、"列一下已连的 mcp"、"看下某台服务器状态"、"直接调 xx 的 mcp 工具" 时触发。
---

# gomcp：接入游戏服 MCP 调试服务

游戏服的 MCP 服务（`yyds/gomcp`）每台机器地址端口都不同。
本 skill 把「校验连通 → 注册 → 提示重连」固化成一步。

## 用法速查

```text
/gomcp                                     列出已注册条目及连通状态
/gomcp <地址:端口> [--name N] [--token T]   注册（需重启会话才生效）
/gomcp <地址:端口> --scope project          注册为项目共享（写入库的 .mcp.json）
/gomcp tools <地址>                        列出该服务器的工具（不注册，立即可用）
/gomcp call <地址> <工具> [--args '<JSON>'] 直接调用工具（不注册，立即可用）
/gomcp remove <名字> [--project]           移除条目（默认只动本机）
```

第一个参数是**子命令名**就整体透传，是**地址**就走注册——两种写法都认。

## 执行流程

**在项目根目录执行**（local 作用域按 cwd 关联本项目）。全程用中文回复。

1. **用户没给地址时** —— 不要猜。列出已注册条目及连通状态，并告诉用户用法
   （gomcp 默认端口 __PORT__）：

   ```bash
   __RUNNER__ .claude/skills/gomcp/connect.py list
   ```

2. **参数以子命令开头**（`connect` / `list` / `remove` / `tools` / `call`）—— 整体透传，
   不要再补 `connect`：

   ```bash
   __RUNNER__ .claude/skills/gomcp/connect.py $ARGUMENTS
   ```

3. **参数是地址**（如 `127.0.0.1:__PORT__`）—— 走注册，原样透传后缀参数
   （`--token` / `--name` / `--scope`）。若用户提到 "测试机" "大家都要用" "共享"，
   提醒可加 `--scope project` 让配置入库共享：

   ```bash
   __RUNNER__ .claude/skills/gomcp/connect.py connect $ARGUMENTS
   ```

   > 只是想看一眼某台服务器的状态/数据，别急着注册——注册要重启会话才生效，
   > 用下面的 `call` 立刻就能拿到结果。

4. **注册成功** —— 简报注册名与地址，并明确提醒：**要重启 Claude Code 会话**（退出重进，
   或 IDE 里 Reload Window）工具才会进来。这一步没法替用户完成。

   不要让用户去 `/mcp` 里 Reconnect——会话启动时就确定了 server 列表，新注册的条目
   压根不在里面，Reconnect 只对已在列表中的条目有效。

5. **失败** —— 脚本已按类型给出原因和处置，**直接转述**，不要另编一套解释。
   校验不通过时不会留下任何注册条目，如实说明。

移除某个条目：

```bash
__RUNNER__ .claude/skills/gomcp/connect.py remove <名字>
```

## 临时用一下某个服务器：直接调，不注册

注册的代价是**必须重启会话**工具才进来。如果只是想看一眼某台服务器
（用户问"测试机什么状态"、"那台机器卡不卡"），不要让用户去注册 + 重启，
直接打它的 HTTP 端点：

```bash
__RUNNER__ .claude/skills/gomcp/connect.py tools <地址>
__RUNNER__ .claude/skills/gomcp/connect.py call <地址> <工具名> [--args '<JSON>']
```

```bash
# 例
... connect.py call 192.168.0.21:__PORT__ server_status
... connect.py call 127.0.0.1:__PORT__ pprof_capture --args "{\"type\":\"cpu\",\"seconds\":10}" --timeout 90
```

不注册、不重启、不在 `/mcp` 列表留痕，服务器暴露的工具全都能调（`tools` 可列出，
具体有哪些取决于该服务注册了什么）。耗时工具（`pprof_capture`）用 `--timeout` 放宽，默认 60 秒。

**什么时候仍该用 `connect` 注册**：这台服务器要**反复长期**使用——注册后工具原生挂在会话里，
不必每次拼命令行，参数也有 schema 提示。一次性排查用 `call` 就够。

地址接受 `127.0.0.1:__PORT__`、`192.168.0.21:__PORT__`、`http://192.168.0.21:__PORT__` 三种写法，**端口必填**。

## 行为约定

- **先校验后注册**：跑一次 MCP `initialize` 握手，不通就报错且**不产生任何条目**——
  注册一个连不上的 server 只会在 `/mcp` 里多一个红叉。
- **名字自动推导**，本地省略 host：`127.0.0.1:__PORT__` → `__APPID__-__PORT__`；
  `192.168.0.21:__PORT__` → `__APPID__-192-168-0-21-__PORT__`。`--name` 可覆盖，但
  **Claude Code 只接受字母、数字、连字符、下划线——不能用中文**（如 `--name __APPID__-local`）。
- **多个并存**：不同地址各占一个条目，可同时接本地和测试服；工具名按
  `mcp__<条目名>__<工具名>` 区分，互不冲突。
- **两种作用域**：

  | | 写到哪 | 谁能用 | 适合 |
  |---|---|---|---|
  | `local`（默认） | 本机 `~/.claude.json` 本项目段 | 只有自己 | 本地私服、同事机器这种只有你要连的地址 |
  | `project` | 项目根 `.mcp.json`（**入版本库**） | 提交后全团队 | 测试机这种大家都要连的地址，免得每人注册一次 |

  `project` 的 `.mcp.json` 要提交才能共享；团队成员首次会看到该 server 处于
  **Pending approval**，批准一次即可。

- **碰 `.mcp.json` 一律要显式带参数**：新增要 `--scope project`，删除要 `remove --project`。
  不带参数的命令只动本机，绝不动这个入库文件。默认 `local` 注册时若撞上同名的项目级条目，
  直接报错让你换名字或显式覆盖，**不会**替你删掉团队配置。
  （`remove --project` 删掉最后一个条目时才会顺带清掉 `.mcp.json` 空壳，删完记得提交。）

- **`--scope project` 拒绝同时传 `--token`**：`.mcp.json` 入库，写进去的 token 等于公开。
  需要 token 的服务器请各自用 `--scope local` 注册；调试服的推荐做法是**干脆不配 token**，
  靠内网 + 只绑内网网卡防护。

## 交互约定

- 地址、端口、token 是自由文本，直接在对话里收。
- 需要**二选一决策**（如"已存在同名条目，是否覆盖？"）时用 `AskUserQuestion`，不要让用户敲 yes/no。

## 排障

| 现象 | 原因与处理 |
|---|---|
| 连接被拒 | 目标服没启动，或它的配置没配 `[mcp].address`。**远程连不上最常见的原因是对方绑了 `127.0.0.1`**（gomcp 默认值，只监听本机），需对方改成 `0.0.0.0` 或具体网卡 IP 后重启 |
| HTTP 401 | 目标服配了 `[mcp].token`，用 `--token` 传同一个值（值在对方的配置文件里） |
| 连接超时 | 检查地址可达性与防火墙 |
| 端口能连上但不是 HTTP/MCP 服务 | 该端口被别的服务占用（换个 `[mcp].address` 端口）／目标服版本还没有 gomcp（需部署带 `yyds/gomcp` 的新版）／目标服没配 `[mcp].address` |
| 注册名不合法 | `--name` 只能用字母、数字、连字符、下划线，中文会被 Claude Code 拒绝 |
| 未找到 claude CLI | 确认装了 Claude Code 且 `claude` 在 PATH（npm 全局装通常在 `%APPDATA%\npm`） |
| `/mcp` 面板里没有刚注册的条目 | **正常现象**。会话启动时就确定了 server 列表，中途新增的要重启 Claude Code 才进来；Reconnect 只对已在列表中的条目有效。用 `connect.py list` 或 `claude mcp list` 可确认注册本身是否成功 |
| 重启后工具仍没出现 | 确认注册写进了当前项目段：`~/.claude.json` 的 `projects` 下应有本项目路径 → `mcpServers` → 条目名。local 作用域按 cwd 关联项目，在别的目录跑 connect 会写到别的项目段 |

<!-- 本文件由 `go run github.com/hwcer/yyds/gomcp/cmd/skill` 生成，模板在 yyds/gomcp/skill/templates。
     要改通用内容请改模板并重新生成；项目专属补充可直接写在本文件里，重新生成时记得带 -force 并手工合回。 -->
