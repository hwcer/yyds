package options

import (
	"strconv"

	"github.com/hwcer/updater"
)

var Setting = struct {
	Renewal  string                        //启用跨天
	GetRole  func(u *updater.Updater) Role //获取角色基础模型，游戏服必须设置
	GetIMax  func(iid int32) (r int64)
	GetIType func(iid int32) (r int32) //替代config.GetIType

}{
	Renewal:  "/role/renewal",
	GetIMax:  getIMaxDefault,
	GetIType: getITypeDefault,
}

func getIMaxDefault(iid int32) (r int64) {
	return 0
}

func getITypeDefault(iid int32) (r int32) {
	if iid < 10 {
		return 0
	}
	s := strconv.Itoa(int(iid))
	v, _ := strconv.Atoi(s[0:2])
	r = int32(v)
	return
}

// Role 角色基础模型
type Role interface {
	GetGuid() string      //游戏账号ID
	GetCreateTime() int64 //角色创建时间
}
