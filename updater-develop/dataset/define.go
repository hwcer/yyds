package dataset

const (
	ItemNameOID = "_id"
	ItemNameVAL = "val"
)

type Model interface {
	GetOID() string //获取OID
	GetIID() int32  //获取IID
}

type ModelGet interface {
	Get(string) (v any, ok bool)
}

// ModelSet 内存写入
//
//	r.(type)==Update 时直接将 r.(Update)写入数据库
//	其他类型  写入{k:r}
type ModelSet interface {
	Set(k string, v any) (r any, ok bool)
}

type ModelClone interface {
	Clone() any
}

type BulkWrite interface {
	Save() error
	Update(data Update, where ...any)
	Insert(documents ...any)
	Delete(where ...any)
}
