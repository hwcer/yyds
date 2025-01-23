package context

import (
	"fmt"
	"github.com/hwcer/cosgo"
	"github.com/hwcer/yyds/options"
	"os"
	"plugin"
)

func init() {
	Register(update)
	options.OAuth.Set(options.ServiceTypeGame, "update", options.OAuthTypeNone)
}

// update 热更新
func update(c *Context) any {
	p := c.GetString("plugin")
	if p == "" {
		return c.Error("plugin empty")
	}
	file := cosgo.Abs(p)
	if _, err := os.Stat(file); err != nil {
		return c.Errorf(0, "热更错误，已经放弃执行热更。\n热更文件:%v\n错误信息:%v\n", file, err)
	}
	plug, err := plugin.Open(file)
	if err != nil {
		return err
	}

	i, err := plug.Lookup("Range")
	if err != nil {
		return err
	}
	r, ok := i.(func(func(string, func(*Context) any) bool))
	if !ok {
		return c.Error("plugin not found func Range")
	}

	s := make(map[string]any)
	r(func(name string, f func(*Context) any) bool {
		s[name] = f
		fmt.Printf("发现函数:%v\n", name)
		return true
	})
	if err = Service.Replace(s); err != nil {
		return err
	}
	return "ok"
}
