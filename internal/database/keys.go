package database

import (
	"fmt"
)

func keyLocalData() []byte {
	return []byte("local.data")
}

func keyClient(pkHash string) []byte {
	return []byte(fmt.Sprintf("clients[%s]", pkHash))
}

func keyChannel(chanID string) []byte {
	return []byte(fmt.Sprintf("channels[%s]", chanID))
}

func keyClientChannel(pkHash string, index uint64) []byte {
	return []byte(fmt.Sprintf("clients[%s].channels[%d]", pkHash, index))
}

func keyClientChannelsCount(pkHash string) []byte {
	return []byte(fmt.Sprintf("clients[%s].channels.count", pkHash))
}

func keyChannelMessage(chanID string, index uint64) []byte {
	return []byte(fmt.Sprintf("channels[%s].messages[%d]", chanID, index))
}

func keyChannelsMessageCount(chanID string) []byte {
	return []byte(fmt.Sprintf("channels[%s].messages.count", chanID))
}

func keyMessage(msgHash string) []byte {
	return []byte(fmt.Sprintf("messages[%s]", msgHash))
}
