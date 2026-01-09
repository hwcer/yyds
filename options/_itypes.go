package options

const (
	ITypeRole       int32 = 10
	ITypeGoods      int32 = 20 //常规资源道具,只有数量无限叠加，仅仅扩展使用，非必须
	ITypeItems      int32 = 21 //无法叠加道具，一般不用
	ITypeTicket     int32 = 22
	ITypeViper      int32 = 23
	ITypeGacha      int32 = 24 //抽卡信息
	ITypeHero       int32 = 40
	ITypeEquip      int32 = 50
	ITypeBuilding   int32 = 60 //城堡功能性建筑物
	ITypeDecoration int32 = 61 //城堡装饰物
	//概率表将在服务器端自动开包客户端不需要处理
	ITypeItemGroup int32 = 80 //物品组
	ITypeItemPacks int32 = 81 //礼包
	//任务活动类
	ITypeTask   int32 = 90
	ITypeDaily  int32 = 96
	ITypeRecord int32 = 97
	ITypeActive int32 = 98 //内置模版活动
	ITypeConfig int32 = 99 //后台配置活动(master.Config)
	//内置,无需全局唯一ID
	ITypeMail    int32 = -1
	ITypeShop    int32 = -2  //商店信息
	ITypeChapter int32 = -11 //剧情章节信息
	ITypeDress   int32 = -12 //地表装修,ITypeDecoration是装饰物,属于道具的一种，ITypeDress是把ITypeDecoration布置在地表的信息
)
