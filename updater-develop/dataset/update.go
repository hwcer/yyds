package dataset

type Update map[string]any

func (d Update) Has(k string) (ok bool) {
	_, ok = d[k]
	return
}
func (d Update) Get(k string) (v any, ok bool) {
	v, ok = d[k]
	return
}
func (d Update) Set(k string, v any) {
	d[k] = v
}

func (d Update) Del(k string) {
	delete(d, k)
}

func (d Update) Merge(from Update) {
	for k, v := range from {
		d[k] = v
	}
}

func NewUpdate(k string, v any) Update {
	r := Update{}
	r[k] = v
	return r
}

func ParseUpdate(src any) Update {
	switch v := src.(type) {
	case map[string]any:
		return v
	case Update:
		return v
	}
	return nil
}
