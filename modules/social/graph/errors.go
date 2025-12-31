package graph

import "github.com/hwcer/cosgo/values"

var ErrorIndex = int32(1300)

var (
	ErrorYourFriendMax   = values.Errorf(ErrorIndex+1, "好友已满，先去删掉一些吧")
	ErrorTargetFriendMax = values.Errorf(ErrorIndex+2, "对方好友已满，无法添加")

	ErrorYourUnfriend   = values.Errorf(ErrorIndex+3, "你已经拉黑了对方")
	ErrorTargetUnfriend = values.Errorf(ErrorIndex+4, "对方已经把你拉黑")

	ErrorAddYourself = values.Errorf(ErrorIndex+5, "Do not add yourself as a friend")

	ErrorAlreadyFriend = values.Errorf(ErrorIndex+6, "already friend")
	ErrorAlreadyFollow = values.Errorf(ErrorIndex+7, "already follow")

	ErrorUserNotExist = values.Errorf(ErrorIndex+8, "User does not exist")
	ErrorNotFans      = values.Errorf(ErrorIndex+8, "User not fans")
)

type Result struct {
	Error  error            //系统错误
	Result map[string]error //批量时 每个用户对应的错误
}

func NewResultError(err error) *Result {
	return &Result{Error: err}
}

func NewResult() *Result {
	return &Result{Result: make(map[string]error)}
}

func (this *Result) Success() (r []string) {
	if this.Error != nil {
		return
	}
	for k, v := range this.Result {
		if v != nil {
			r = append(r, k)
		}
	}
	return
}

// Failed 查询特定用户失败原因
func (this *Result) Failed(uid string) error {
	if this.Error != nil {
		return this.Error
	}
	return this.Result[uid]
}
