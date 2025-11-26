package errors

import (
	"github.com/hwcer/cosgo/values"
)

var (
	ErrLogin            = values.Errorf(1, "not login")         //请重新登录
	ErrLocked           = values.Errorf(2, "wait a minute")     //请求太快等一会
	ErrReplaced         = values.Errorf(3, "Sign in elsewhere") //其他地方登录,被顶号
	ErrNotSelectRole    = values.Errorf(4, "not select role")   //请先选择角色
	ErrServerLimit      = values.Errorf(5, "server role limit") //服务器创角已满
	ErrMasterEmpty      = values.Errorf(6, "master url is empty")
	ErrRoleNotExist     = values.Errorf(10, "role not exist")            // 角色不存在
	ErrLoginWaiting     = values.Errorf(11, "Wait a moment")             //正在释放数据,需要等一会再登录
	ErrNeedResetSession = values.Errorf(12, "need reset session")        //跨天需要特殊处理
	ErrNeedGameMaster   = values.Errorf(13, "GM permission is required") //需要GM权限
	ErrNotOnline        = values.Errorf(14, "user not online")           //不在线

	ErrLoginAgain    = values.Errorf(101, "please login again") //需要重新登录
	ErrLoginDisabled = values.Errorf(102, "disabled")           //账号禁用
	ErrDataNotExists = values.Errorf(104, "data not exists")    //数据库数据不存在
	ErrPlayerMax     = values.Errorf(105, "player max")         //房间已满
	ErrDataExists    = values.Errorf(106, "data exists")        //数据已经存在
	ErrItemNotEnough = values.Errorf(202, "item not enough")    //道具,材料不足
	ErrTargetLimit   = values.Errorf(203, "target limit")       //任务目标未达成
	ErrPreTaskLimit  = values.Errorf(204, "pre task limit")     //前置任务没完成
	ErrArgEmpty      = values.Errorf(400, "args empty")         //参数不能为空
	ErrActiveDisable = values.Errorf(401, "active disable")     //活动未开始
	ErrActiveExpired = values.Errorf(402, "active expired")     //活动已经结束
	ErrConfigEmpty   = values.Errorf(403, "config empty")

	ErrRoleIsBan = values.Errorf(500, "user is ban")
)

//func init() {
//	Register(ErrLogin, ErrLocked, ErrNotSelectRole, ErrServerLimit, ErrMasterEmpty, ErrRoleNotExist, ErrLoginWaiting, ErrNeedResetSession)
//	Register(ErrLoginAgain, ErrLoginDisabled, ErrDataNotExists, ErrPlayerMax, ErrDataExists, ErrItemNotEnough, ErrTargetLimit, ErrPreTaskLimit, ErrArgEmpty, ErrActiveDisable, ErrActiveExpired, ErrConfigEmpty)
//}
