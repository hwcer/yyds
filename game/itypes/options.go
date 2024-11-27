package itypes

import (
	"github.com/hwcer/cosgo/random"
	"github.com/hwcer/updater"
)

type ItemsGroupConfig interface {
	GetKey() int32
	GetNum() int32
}

type ItemsPacksConfig interface {
	ItemsGroupConfig
	GetVal() int32
}

type ItemsTicketConfig interface {
	GetDot() []int32
	GetLimit() []int32
	GetCycle() []int32
}

var Options = struct {
	GetItemsTicketConfig func(int32) ItemsTicketConfig

	GetItemsPacksConfig func(int32) []ItemsPacksConfig //获取物品包配置
	GetItemsGroupConfig func(int32) ItemsGroupConfig   //获取物品组配置
	GetItemsGroupRandom func(int32) *random.Random     //获取物品组概率表

	RoleVerify  func(u *updater.Updater, exp int64) (newExp int64)    //获得经验时进行检查
	RoleUpgrade func(u *updater.Updater, lv, exp int64) (newLv int64) //判断升级，返回新的等级

}{}
