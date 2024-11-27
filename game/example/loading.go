package main

import "github.com/hwcer/cosgo"

//演示加载module中的模块

func init() {
	cosgo.On(cosgo.EventTypLoader, loading)
}

func loading() error {

	return nil
}
