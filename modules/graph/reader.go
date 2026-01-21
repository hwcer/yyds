package graph

import "github.com/hwcer/cosgo/values"

type Reader struct {
	sg *Graph
	p  *Player
}

func (this *Reader) Uid() string {
	return this.p.Uid()
}
func (this *Reader) Get(key string) any {
	return this.p.Get(key)
}

func (this *Reader) GetInt(k string) int {
	return this.p.GetInt(k)
}

func (this *Reader) GetInt32(k string) int32 {
	return this.p.GetInt32(k)
}

func (this *Reader) GetInt64(k string) int64 {
	return this.p.GetInt64(k)
}

func (this *Reader) GetFloat32(k string) float32 {
	return this.p.GetFloat32(k)
}

func (this *Reader) GetFloat64(k string) (r float64) {
	return this.p.GetFloat64(k)
}
func (this *Reader) GetString(k string) (r string) {
	return this.p.GetString(k)
}

func (this *Reader) Values() values.Values {
	return this.p.Values.Clone()
}

func (this *Reader) Friend(fid string) *Getter {
	fri := this.p.friends[fid]
	if fri == nil {
		return nil
	}
	return &Getter{sg: this.sg, fid: fid, fri: fri}
}

type Getter struct {
	sg  *Graph
	fid string
	fri *Friend
}

func (this *Getter) Fid() string {
	return this.fid
}

func (this *Getter) Get(key string) any {
	return this.fri.Get(key)
}

func (this *Getter) GetInt(k string) int {
	return this.fri.GetInt(k)
}

func (this *Getter) GetInt32(k string) int32 {
	return this.fri.GetInt32(k)
}

func (this *Getter) GetInt64(k string) int64 {
	return this.fri.GetInt64(k)
}

func (this *Getter) GetFloat32(k string) float32 {
	return this.fri.GetFloat32(k)
}

func (this *Getter) GetFloat64(k string) (r float64) {
	return this.fri.GetFloat64(k)
}
func (this *Getter) GetString(k string) (r string) {
	return this.fri.GetString(k)
}

func (this *Getter) Values() values.Values {
	return this.fri.Clone()
}

// Update 好有动态
func (this *Getter) Update() int64 {
	p := this.sg.nodes[this.fid]
	if p == nil {
		return 0
	}
	return p.Update
}
