package database

import (
	"github.com/number571/fuckoff-gov/internal/models"
)

type IDatabase interface {
	GetLocalData() (*models.LocalData, error)
	SetLocalData(localData *models.LocalData) error

	SetClient(clientInfo *models.ClientInfo) error
	GetClient(pkHash string) (*models.ClientInfo, error)

	SetChannel(channelInfo *models.ChannelInfo) error
	GetChannel(chanID string) (*models.ChannelInfo, error)

	GetCountClientChannels(pkHash string) (uint64, error)
	GetClientChanIDByIndex(pkHash string, index uint64) (string, error)

	GetCountChannelMessages(chanID string) (uint64, error)
	GetChannelMessageHashByIndex(chanID string, index uint64) (string, error)

	AddChannelMessage(messageInfo *models.MessageInfo) error
	GetMessage(msgHash string) (*models.MessageInfo, error)
}
