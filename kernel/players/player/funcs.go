package player

type Arr []int32

func (a Arr) Has(v int32) bool {
	for _, vv := range a {
		if vv == v {
			return true
		}
	}
	return false
}
