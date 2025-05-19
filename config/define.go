package config

type iMax interface {
	GetIMax() int32
}
type iType interface {
	GetIType() int32
}
type iName interface {
	GetName() string
}

// Handle 检查或者预处理接口
type Handle interface {
	Handle(c *CS, d any)         //配置预处理
	Verify(c *CS, d any) []error //配置检查
}
