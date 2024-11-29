package config

//配置加载管理

var handles []handle

// 检查或者预处理接口
type handle interface {
	Handle(c *Config, d any)         //配置预处理
	Verify(c *Config, d any) []error //配置检查
}

type Default struct {
}

func (this *Default) Handle(*Config, any) {
}

func (this *Default) Verify(*Config, any) (errs []error) {
	return
}

// Register 注册配置检查程序
func Register(i ...handle) {
	handles = append(handles, i...)
}
