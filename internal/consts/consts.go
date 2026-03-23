package consts

const (
	Version = "v0.0.1"
)

const (
	WorkSizeClient  = 1
	WorkSizeChannel = 1
	WorkSizeMessage = 1
)

const (
	MaxMessageSize  = (1 << 20) // 1MiB
	MaxNickNameSize = 64        // bytes
	MaxFileNameSize = 256       // bytes
)

const (
	CountMessagesPerPage = 1
)
