package itype

import (
	"github.com/hwcer/cosgo/random"
	"testing"
)

var itemsRandomConfigs = map[int32]itemRandomConfig{}

func TestRoleRandom(t *testing.T) {
	ItemsGroup.Random = parseItemGroup_test
	ItemsPacks.Random = parseItemPacks_test
	//模拟配置列表
	itemsRandomConfigs[80001] = itemRandomConfig{Id: 80001, Key: 80002, Num: 2, Val: 100}
	itemsRandomConfigs[80002] = itemRandomConfig{Id: 80002, Key: 20001, Num: 2, Val: 100}
}

type itemRandomConfig struct {
	Id  int32
	Val int32 //权重或者概率
	Key int32
	Num int32
}

func (i itemRandomConfig) GetVal() int32 {
	return i.Val
}

func (i itemRandomConfig) GetKey() int32 {
	return i.Key
}
func (i itemRandomConfig) GetNum() int32 {
	return i.Num
}

// ParseItemPacks 独立概率
func parseItemPacks_test(k, v int32) map[int32]int32 {
	r := map[int32]int32{}
	//configs := Options.GetItemsPacksConfig(k)
	//if configs == nil {
	//	return r
	//}
	configs := itemsRandomConfigs
	for i := 0; i < int(v); i++ {
		for _, c := range configs {
			if random.Probability(c.GetVal()) {
				r[c.GetKey()] = c.GetNum()
			}
		}
	}
	return r
}

func parseItemGroup_test(k, v int32) map[int32]int32 {
	r := map[int32]int32{}
	w := random.New(nil)
	for i, j := range itemsRandomConfigs {
		w.Add(i, j.Val)
	}

	for i := 0; i < int(v); i++ {
		if x := w.Roll(); x >= 0 {
			c := itemsRandomConfigs[x]
			r[c.GetKey()] = c.GetNum()
		}
	}
	return r
}
