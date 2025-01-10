package operator

type Types uint8 //Cache act type
const (
	TypesNone     Types = 0  //无意义
	TypesAdd      Types = 1  //添加
	TypesSub      Types = 2  //扣除
	TypesSet      Types = 3  //set
	TypesDel      Types = 4  //del
	TypesNew      Types = 5  //新对象,等同于add,但是装备之类不能叠加时，就是会走NEW生成新对象
	TypesMax      Types = 10 //最大值写入，最终转换成set或者drop
	TypesMin      Types = 11 //最小值写入，最终转换成set或者drop
	TypesDrop     Types = 90 //抛弃不执行任何操作
	TypesResolve  Types = 91 //自动分解
	TypesOverflow Types = 92 //道具已满使用其他方式(邮件)转发
)

func (at Types) IsValid() bool {
	return at == TypesAdd || at == TypesSub || at == TypesSet || at == TypesDel || at == TypesNew
}

func (at Types) MustSelect() bool {
	return at == TypesAdd || at == TypesSub || at == TypesMax || at == TypesMin
}

// MustNumber 必须是正整数的操作
func (at Types) MustNumber() bool {
	return at == TypesAdd || at == TypesSub || at == TypesMax || at == TypesMin
}

func (at Types) ToString() string {
	switch at {
	case TypesAdd:
		return "Add"
	case TypesSub:
		return "Sub"
	case TypesSet:
		return "Set"
	case TypesDel:
		return "Del"
	case TypesNew:
		return "New"
	case TypesResolve:
		return "Resolve"
	case TypesMax:
		return "Max"
	case TypesMin:
		return "Min"
	case TypesDrop:
		return "Drop"
	default:
		return "unknown"
	}
}
