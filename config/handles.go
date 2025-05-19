package config

var handles []Handle

// Register 注册配置检查程序
func Register(i ...Handle) {
	handles = append(handles, i...)
}
