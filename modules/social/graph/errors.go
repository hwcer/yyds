package graph

import "github.com/hwcer/cosgo/values"

var (
	ErrorYourFriendMax   = values.Errorf(1300, "好友已满，先去删掉一些吧")
	ErrorTargetFriendMax = values.Errorf(1301, "对方好友已满，无法添加")
)
