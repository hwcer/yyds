// Command skill 把 gomcp 配套的 Claude Code skill 生成到业务项目里。
//
//	go run github.com/hwcer/yyds/gomcp/cmd/skill -appid myapp
//	go run github.com/hwcer/yyds/gomcp/cmd/skill -appid myapp -o .claude/skills/gomcp -force
//
// 默认不覆盖已存在的文件：SKILL.md 常被各项目改成自己的样子，
// 重新生成时应当由人来决定怎么合，而不是被静默冲掉。
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/hwcer/yyds/gomcp/skill"
)

func main() {
	var (
		opts  skill.Options
		out   string
		force bool
	)
	flag.StringVar(&opts.Appid, "appid", "", "注册名前缀,通常取业务 appid(如 elf → 本地服务器注册成 elf-8300);必填")
	flag.StringVar(&opts.Port, "port", "", "文档示例里的默认端口(默认 "+skill.DefaultPort+")")
	flag.StringVar(&opts.Runner, "runner", "", "执行 connect.py 的命令前缀(默认 "+skill.DefaultRunner+");项目有统一运行入口时用它覆盖")
	flag.StringVar(&out, "o", filepath.Join(".claude", "skills", "gomcp"), "输出目录")
	flag.BoolVar(&force, "force", false, "覆盖已存在的文件")
	flag.Parse()

	if err := run(opts, out, force); err != nil {
		fmt.Fprintln(os.Stderr, "生成失败:", err)
		os.Exit(1)
	}
}

func run(opts skill.Options, out string, force bool) error {
	files, err := skill.Files(opts)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(out, 0o755); err != nil {
		return err
	}

	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names) //输出顺序稳定,便于比对两次生成结果

	var skipped []string
	for _, name := range names {
		dst := filepath.Join(out, filepath.FromSlash(name))
		if !force {
			if _, err = os.Stat(dst); err == nil {
				skipped = append(skipped, dst)
				continue
			} else if !os.IsNotExist(err) {
				return err
			}
		}
		if err = os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err = os.WriteFile(dst, files[name], 0o644); err != nil {
			return err
		}
		fmt.Println("written  ", dst)
	}

	for _, dst := range skipped {
		fmt.Println("skipped  ", dst, "(已存在,加 -force 覆盖)")
	}
	if len(skipped) == len(names) {
		return nil
	}
	fmt.Printf("\n生成完毕。skill 名为 gomcp,在项目根目录敲 /gomcp 即可调用;\n"+
		"本地服务器会注册成 %s-%s 这样的条目名。\n", opts.Appid, portOr(opts.Port))
	return nil
}

func portOr(p string) string {
	if p == "" {
		return skill.DefaultPort
	}
	return p
}
