package updater

import (
	"fmt"
	"github.com/hwcer/cosgo/schema"
	"sort"
)

type Parser int8

const (
	ParserTypeValues     Parser = iota //Map[string]int64模式
	ParserTypeDocument                 //Document 单文档模式
	ParserTypeCollection               //Collection 文档集合模式
)

type handleFunc func(updater *Updater, model any) Handle

var handles = make(map[Parser]handleFunc)

func init() {
	NewHandle(ParserTypeValues, NewValues)
	NewHandle(ParserTypeDocument, NewDocument)
	NewHandle(ParserTypeCollection, NewCollection)
}

type TableOrder interface {
	TableOrder() int32
}

type ModelLoading interface {
	Loading() RAMType
}

// NewHandle 注册新解析器
func NewHandle(name Parser, f handleFunc) {
	handles[name] = f
}

var modelsRank []*Model
var modelsDict = make(map[int32]*Model)
var itypesDict = make(map[int32]IType) //ITypeId = IType

type Model struct {
	ram     RAMType
	name    string
	model   any
	parser  Parser
	order   int32   //倒序排列
	loading RAMType //加载时内存模式
}

func ITypes(f func(int32, IType) bool) {
	for k, it := range itypesDict {
		if !f(k, it) {
			break
		}
	}
}
func Models(f func(int32, any) bool) {
	for k, m := range itypesDict {
		if !f(k, m) {
			break
		}
	}
}
func Register(parser Parser, ram RAMType, model any, itypes ...IType) error {
	if _, ok := handles[parser]; !ok {
		return fmt.Errorf("parser unknown:%v", parser)
	}
	mod := &Model{ram: ram, model: model, parser: parser}
	if t, ok := model.(schema.Tabler); ok {
		mod.name = t.TableName()
	} else {
		mod.name = schema.Kind(model).Name()
	}
	if o, ok := model.(TableOrder); ok {
		mod.order = o.TableOrder()
	} else {
		mod.order = -1
	}
	if o, ok := model.(ModelLoading); ok {
		mod.loading = o.Loading()
	} else {
		mod.loading = RAMTypeNone
	}
	modelsRank = append(modelsRank, mod)
	sort.SliceStable(modelsRank, func(i, j int) bool {
		return modelsRank[i].order > modelsRank[j].order
	})

	for _, it := range itypes {
		if parser == ParserTypeCollection {
			it = it.(ITypeCollection)
		}
		id := it.ID()
		if _, ok := modelsDict[id]; ok {
			return fmt.Errorf("model IType(%v)已经存在:%v", it, mod.name)
		}
		modelsDict[id] = mod
		itypesDict[id] = it
	}
	return nil
}
