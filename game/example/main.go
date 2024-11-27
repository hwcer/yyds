package main

import (
	"fmt"
	"github.com/hwcer/cosgo"
	"server/config"
	"server/game"
	"server/service/gate"
	"server/share"
)

func main() {
	cosgo.SetBanner(banner)
	cosgo.Use(share.New())
	cosgo.Use(config.New())
	cosgo.Use(game.New())
	cosgo.Use(gate.New())
	cosgo.Start(true)
}

func banner() {
	str := "\n大威天龙，大罗法咒，般若诸佛，般若巴嘛空。\n"
	fmt.Printf(str)
}
