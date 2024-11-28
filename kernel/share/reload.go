package share

import (
	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosgo/options"
)

// loadOptions 加载配置
func reload() (err error) {
	//logger.Info("reload config file")
	err = options.Initialize(func() error {
		return cosgo.Config.Unmarshal(options.Options)
	})
	return
}

/*
func loadMasterSetting() (err error) {
	Options.Appid = cosgo.Config.GetString(FlagNameAppid)
	Options.Secret = cosgo.Config.GetString(FlagNameSecret)
	Options.Master = cosgo.Config.GetString(FlagNameMaster)
	if Options.Master == "" {
		return
	}
	//Master.SetUrl(masterUrl)
	//获取options
	data := make(map[string]interface{})
	//data["sid"] = ServerId()
	reply := make(values.Values)
	if err = Master.Post(MasterApiTypeServiceStart, data, &reply); err != nil {
		return
	}

	//Options.localAddress = reply.GetString("ip")
	//Options.Notify = reply.GetString("notify")

	if c := reply.GetString("toml"); c != "" {
		if _, err = toml.Decode(c, Options); err != nil {
			return
		}
	}
	return
}
*/
