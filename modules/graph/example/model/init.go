package model

import (
	"errors"

	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosmo"
)

var (
	db     *cosmo.DB
	models []any
)

type Handle func(uids []string) (map[string]Player, error)

func init() {
	cosgo.On(cosgo.EventTypLoaded, func() error {
		for _, model := range models {
			db.Register(model)
		}
		return nil
	})
}

func DB() *cosmo.DB {
	return db
}

type Player interface {
	GetUid() string
}

var GetPlayers Handle = func(uids []string) (map[string]Player, error) {
	return nil, errors.New("请配置model.GetPlayers")
}

func SetPlayers(gp Handle) {
	GetPlayers = gp
}
func SetDatabase(mongo *cosmo.DB) {
	db = mongo
}

func Register(i ...any) {
	models = append(models, i...)
}
