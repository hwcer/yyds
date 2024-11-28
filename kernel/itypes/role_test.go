package itypes

import (
	"github.com/hwcer/updater"
	"testing"
)

func TestRole(t *testing.T) {
	Role.Upgrade = roleUpgradeHandle{}

}

type roleUpgradeHandle struct {
}

// 获得经验时进行检查
func (roleUpgradeHandle) Verify(u *updater.Updater, exp int64) (newExp int64) {
	return exp
}

// 判断升级，返回新的等级
func (roleUpgradeHandle) Submit(u *updater.Updater, lv, exp int64) (newLv int64) {
	return lv + 1
}
