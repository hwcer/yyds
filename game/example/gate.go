package main

import (
	"github.com/hwcer/cosgo"
	"server/service/gate"
	"server/share"
)

func main() {
	cosgo.SetBanner(banner)
	cosgo.Use(share.New())
	cosgo.Use(gate.New())
	cosgo.Start(true)
}
