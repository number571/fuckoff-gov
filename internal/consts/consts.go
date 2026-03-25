package consts

const (
	Version = "v0.0.1"
)

const (
	WorkSizeClient  = 35
	WorkSizeChannel = 24
	WorkSizeMessage = 22
)

const (
	MaxMessageSize  = (1 << 20) // 1MiB
	MaxChannelName  = 64        // bytes
	MaxNickNameSize = 64        // bytes
	MaxFileNameSize = 256       // bytes
)

const (
	CountMessagesPerPage = 25
)

const (
	HeaderAuthTask  = "Auth-Task"
	HeaderAuthToken = "Auth-Token"
)
