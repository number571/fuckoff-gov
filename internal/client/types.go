package client

import (
	"context"

	"github.com/number571/fuckoff-gov/internal/models"
	"github.com/number571/go-peer/pkg/crypto/asymmetric"
)

type IEncoder interface {
	InitClient() *models.ClientInfo
	InitChannel(name string, pubKeys []asymmetric.IPubKey) (*models.ChannelInfo, error)
	PushMessage(chanID string, key []byte, msgBody *models.MessageBody) (*models.MessageInfo, error)
}

type IDecoder interface {
	ClientInfo(clientInfo *models.ClientInfo) (asymmetric.IPubKey, error)
	ChannelInfo(pubKey asymmetric.IPubKey, channelInfo *models.ChannelInfo) ([]byte, string, error)
	MessageInfo(pubKey asymmetric.IPubKey, key []byte, messageInfo *models.MessageInfo) (*models.MessageBody, error)
}

type IClient interface {
	// NON AUTH METHODS:

	Ping(ctx context.Context) error
	Auth(ctx context.Context) error

	InitClient(ctx context.Context, clientInfo *models.ClientInfo) error
	LoadClient(ctx context.Context, pkhash string) (*models.ClientInfo, error)

	// AUTH METHODS:

	CountChannels(ctx context.Context) (uint64, error)
	ListenChannel(ctx context.Context, index uint64) (*models.ChannelInfo, error)

	InitChannel(ctx context.Context, channelInfo *models.ChannelInfo) error
	LoadChannel(ctx context.Context, chanID string) (*models.ChannelInfo, error)

	PushMessage(ctx context.Context, messageInfo *models.MessageInfo) error
	LoadMessage(ctx context.Context, mhash string) (*models.MessageInfo, error)

	CountMessages(ctx context.Context, chanID string) (uint64, error)
	ListenMessage(ctx context.Context, chanID string, index uint64) (*models.MessageInfo, error)
}
