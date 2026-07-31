// Package skill 提供 gomcp 调试服务配套的 Claude Code skill 模板。
//
// 接了 yyds/gomcp 的业务项目，用下面这行在自己仓库里生成一份可直接用的 skill，
// 不必从别的项目手工拷贝、也不必自己写一遍连接逻辑：
//
//	go run github.com/hwcer/yyds/gomcp/cmd/skill -appid myapp
//
// 本包**不 import gomcp 主包**：主包会把 cosgo / mongo / rpcx 等运行时依赖拖进来，
// 而生成 skill 只是个开发期动作，让 go run 只编译标准库 + embed 才够快。
// 生成出来的 connect.py 同样只用 Python 标准库，不需要虚拟环境或 pip 安装。
package skill

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"
)

//go:embed templates
var templates embed.FS

const templateDir = "templates"

// DefaultPort gomcp 的惯用端口，仅用于生成文档里的示例地址
const DefaultPort = "8300"

// DefaultRunner 执行 connect.py 的命令前缀。
// connect.py 只用标准库，系统 python 直接能跑；宿主项目若有统一的运行入口
// （虚拟环境包装脚本之类），生成时用 -runner 覆盖。
const DefaultRunner = "python"

// 注册名会进 Claude Code 的 mcp server 名字，那边只接受字母数字连字符下划线
var appidPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// Options 渲染参数
type Options struct {
	// Appid 注册名前缀，通常取业务的 appid。
	// 例如 "elf" 会让本地服务器注册成 elf-8300，便于 list/remove 只管本 skill 建的条目。
	Appid string
	// Port 文档示例里用的默认端口，留空取 DefaultPort
	Port string
	// Runner 执行 connect.py 的命令前缀，留空取 DefaultRunner
	Runner string
}

func (o *Options) normalize() error {
	o.Appid = strings.TrimSpace(o.Appid)
	if o.Appid == "" {
		return fmt.Errorf("appid 不能为空:它决定注册名前缀(如 elf → elf-%s)", DefaultPort)
	}
	if !appidPattern.MatchString(o.Appid) {
		return fmt.Errorf("appid %q 不合法:Claude Code 的 mcp server 名只接受字母、数字、连字符、下划线", o.Appid)
	}
	if o.Port == "" {
		o.Port = DefaultPort
	}
	if o.Runner == "" {
		o.Runner = DefaultRunner
	}
	return nil
}

// Files 渲染出全部 skill 文件，key 是相对 skill 目录的路径（SKILL.md / connect.py）。
//
// 占位符用 __NAME__ 而不是 Go template：connect.py 里有 f-string 的 {{ 转义，
// 走 text/template 会解析失败。
func Files(o Options) (map[string][]byte, error) {
	if err := o.normalize(); err != nil {
		return nil, err
	}
	replacer := strings.NewReplacer(
		"__APPID__", o.Appid,
		"__PORT__", o.Port,
		"__RUNNER__", o.Runner,
	)
	out := make(map[string][]byte)
	err := fs.WalkDir(templates, templateDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := templates.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepathRel(templateDir, p)
		if err != nil {
			return err
		}
		out[rel] = []byte(replacer.Replace(string(b)))
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("模板为空:embed 没有取到 %s 下的文件", templateDir)
	}
	return out, nil
}

// filepathRel 只处理 embed.FS 的斜杠路径，不用 path/filepath（那个在 Windows 上会拼反斜杠）
func filepathRel(base, target string) (string, error) {
	base = path.Clean(base) + "/"
	if !strings.HasPrefix(target, base) {
		return "", fmt.Errorf("路径 %q 不在 %q 下", target, base)
	}
	return strings.TrimPrefix(target, base), nil
}
