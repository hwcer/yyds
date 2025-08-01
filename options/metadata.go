package options

const (
	ServiceMetadataUID       = "uid"
	ServiceMetadataGUID      = "guid"
	ServiceMetadataMaster    = "gmr" //GM身份
	ServiceMetadataServerId  = "sid"
	ServiceMetadataRequestId = "rid"

	ServiceSocketId      = "_sock_id"
	ServiceMessagePath   = "_msg_path"
	ServiceMessageRoom   = "_msg_room"
	ServiceMessageIgnore = "_msg_ignore"

	ServicePlayerOAuth   = "_player_oauth"
	ServicePlayerLogout  = "_player_logout"
	ServicePlayerGateway = "_player_gateway"

	ServicePlayerRoomJoin  = "player.join"      //已经加入的房间
	ServicePlayerRoomLeave = "player.leave"     //离开房间
	ServicePlayerSelector  = "service.selector" //服务器重定向
	
)
