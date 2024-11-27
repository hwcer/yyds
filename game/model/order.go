package model

type OrderStatus int8

// 订单
const (
	OrderStatusCREATE  OrderStatus = 0 //新订单
	OrderStatusPAYMENT OrderStatus = 7 //已付款未发货
	OrderStatusCANCEL  OrderStatus = 8 //取消
	OrderStatusSUCCESS OrderStatus = 9 //发货成功
)

func init() {
	Register(&Order{})
}

type Order struct {
	Model  `bson:"inline"`
	Create int64  `json:"create" bson:"create" `            //订单创建时间
	Expire int64  `json:"expire" bson:"expire" `            //过期时间
	Trade  string `json:"trade" bson:"trade" index:"name:"` //平台订单号，用于对账
	//Goods    int32       `json:"goods" bson:"goods"`               //payment ID
	Status   OrderStatus `json:"status" bson:"status"`     //状态
	Amount   float32     `json:"amount" bson:"amount"`     //订单金额(元，默认货币)
	Receive  float32     `json:"receive" bson:"receive"`   //实际到账金额(元),平台折扣，代金券之类会抵消部分金额
	Currency string      `json:"currency" bson:"currency"` //实际支付货币类型，参考百度"货币代码"，默认 CNY = 人民
}

// TableName 数据库表名
func (this *Order) TableName() string {
	return "orders"
}
