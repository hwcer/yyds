package updater

import (
	"github.com/hwcer/updater/operator"
)

type RAMType int8

const (
	RAMTypeNone   RAMType = iota //实时读写数据
	RAMTypeMaybe                 //按需读写
	RAMTypeAlways                //内存运行
)

// 通过MODEL直接获取IType
type modelIType interface {
	IType(iid int32) int32
}

type stmHandleOptCreate func(t operator.Types, k any, v int64, r any)
type stmHandleDataExist func(k any) bool

type statement struct {
	ram             RAMType
	keys            Keys
	cache           []*operator.Operator
	loader          bool                 //是否已经完成加载
	Updater         *Updater             //Updater
	operator        []*operator.Operator //操作
	handleOptCreate stmHandleOptCreate   //Create
	handleDataExist stmHandleDataExist   //查询数据集中是否存在
}

func newStatement(u *Updater, opt stmHandleOptCreate, exist stmHandleDataExist) *statement {
	return &statement{handleOptCreate: opt, handleDataExist: exist, Updater: u}
}

func (stmt *statement) stmt() *statement {
	return stmt
}

// Has 查询key(DBName)是否已经初始化
func (stmt *statement) has(key any) bool {
	if stmt.ram == RAMTypeAlways {
		return true
	}
	if stmt.keys != nil && stmt.keys.Has(key) {
		return true
	}
	return stmt.handleDataExist(key)
}

func (stmt *statement) reset() {
	//if stmt.values == nil {
	//	stmt.values = map[any]int64{}
	//}
	//if stmt.keys == nil && stmt.ram != RAMTypeAlways {
	//	stmt.keys = Keys{}
	//}
}

// 每一个执行时都会执行 release
func (stmt *statement) release() {
	stmt.keys = nil
	stmt.operator = nil
}

// date 执行Data 后操作
func (stmt *statement) date() {
	stmt.keys = nil
}

// verify 执行verify后操作
func (stmt *statement) verify() {
	if len(stmt.operator) == 0 {
		return
	}
	if stmt.cache == nil {
		stmt.cache = make([]*operator.Operator, 0, len(stmt.operator))
	}
	for _, v := range stmt.operator {
		if Config.Filter(v) {
			stmt.cache = append(stmt.cache, v)
		}
	}
	stmt.operator = nil
}

func (stmt *statement) submit() {
	if len(stmt.cache) > 0 {
		stmt.Updater.dirty = append(stmt.Updater.dirty, stmt.cache...)
		stmt.cache = nil
	}
}

func (this *statement) Loader() bool {
	return this.loader
}

func (stmt *statement) Select(key any) {
	if stmt.has(key) {
		return
	}
	if stmt.keys == nil {
		stmt.keys = Keys{}
	}
	stmt.keys.Select(key)
	stmt.Updater.changed = true
}

func (stmt *statement) Errorf(format any, args ...any) error {
	return stmt.Updater.Errorf(format, args...)
}

// Operator 直接调用有问题
func (stmt *statement) Operator(c *operator.Operator, before ...bool) {
	if len(before) > 0 && before[0] {
		stmt.operator = append([]*operator.Operator{c}, stmt.operator...)
	} else {
		stmt.operator = append(stmt.operator, c)
	}
	stmt.Updater.operated = true
}

func (stmt *statement) Add(k any, v int32) {
	if v <= 0 {
		return
	}
	stmt.handleOptCreate(operator.TypesAdd, k, int64(v), nil)
}

func (stmt *statement) Sub(k any, v int32) {
	if v <= 0 {
		return
	}
	stmt.handleOptCreate(operator.TypesSub, k, int64(v), nil)
}

func (stmt *statement) Max(k any, v int64) {
	stmt.handleOptCreate(operator.TypesMax, k, v, nil)
}

func (stmt *statement) Min(k any, v int64) {
	stmt.handleOptCreate(operator.TypesMin, k, v, nil)
}

func (stmt *statement) Del(k any) {
	stmt.handleOptCreate(operator.TypesDel, k, 0, nil)
}
