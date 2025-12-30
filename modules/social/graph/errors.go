package graph

import "github.com/hwcer/cosgo/values"

var (
	ErrorYourFriendMax   = values.Errorf(1300, "好友已满，先去删掉一些吧")
	ErrorTargetFriendMax = values.Errorf(1301, "对方好友已满，无法添加")

	ErrorYourUnfriend   = values.Errorf(1302, "你已经拉黑了对方")
	ErrorTargetUnfriend = values.Errorf(1303, "对方已经把你拉黑")

	ErrorAddYourself = values.Errorf(1304, "Do not add yourself as a friend")

	ErrorAlreadyFriend = values.Errorf(1305, "already friend")
	ErrorAlreadyFollow = values.Errorf(1305, "already follow")
)
