package updater

import "github.com/hwcer/updater/operator"

type Handle interface {
	Del(k any)                                      //删除道具
	Get(k any) any                                  //获取值
	Val(k any) int64                                //获取val值
	Add(k any, v int32)                             //自增v
	Sub(k any, v int32)                             //扣除v
	Max(k any, v int64)                             //如果大于原来的值就写入
	Min(k any, v int64)                             //如果小于于原来的值就写入
	Set(k any, v ...any)                            //设置v值
	Data() error                                    //非内存模式获取数据库中的数据
	Select(keys ...any)                             //非内存模式时获取特定道具
	Parser() Parser                                 //解析模型
	Operator(op *operator.Operator, before ...bool) //直接添加并执行封装好的Operator,不会触发任何事件
	IType(int32) IType                              //根据iid获取IType
	stmt() *statement                               //获取核心
	save() error                                    //保存所有数据
	reset()                                         //运行时开始时
	loading(RAMType) error                          //构造方法,load 是否需要加载数据库数据
	release()                                       //运行时释放缓存信息,并返回所有操作过程
	destroy() error                                 //同步所有数据到数据库,手动同步,或者销毁时执行
	submit() error                                  //即时同步,提交所有操作,缓存生效,同步数据库
	verify() error                                  //验证数据,执行过程的数据开始按顺序生效,但不会修改缓存
}
