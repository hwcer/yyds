package graph

type FollowResult int8

const (
	FollowResultNone     FollowResult = 0  //成为对方粉丝
	FollowResultFriend   FollowResult = 1  //直接成为好友
	FollowResultUnfriend FollowResult = -1 //黑粉无法加好友
	FollowResultFailure  FollowResult = -2 //添加失败
)
