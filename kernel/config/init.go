package config

//配置加载管理

var hvs []hv

// 检查或者预处理接口
type hv interface {
	Handle(any)         //配置预处理
	Verify(any) []error //配置检查
}

// Register 注册配置检查程序
func Register(i ...hv) {
	hvs = append(hvs, i...)
}
