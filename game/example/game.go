package main

import (
	"github.com/hwcer/cosgo"
	"server/config"
	"server/game"
	"server/share"
)

func main() {
	cosgo.SetBanner(banner)
	cosgo.Use(share.New())
	cosgo.Use(config.New())
	cosgo.Use(game.New())
	cosgo.Start(true)
}
