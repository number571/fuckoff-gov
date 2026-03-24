package database

import (
	"encoding/json"
	"errors"
	"sync"

	"github.com/number571/fuckoff-gov/internal/models"
	"github.com/number571/go-peer/pkg/crypto/asymmetric"
	"github.com/number571/go-peer/pkg/encoding"
	"github.com/number571/go-peer/pkg/storage/database"
)

type sDatabase struct {
	mu *sync.RWMutex
	db database.IKVDatabase
}

func OpenDatabase(path string) (IDatabase, error) {
	db, err := database.NewKVDatabase(path)
	if err != nil {
		return nil, err
	}
	return &sDatabase{
		mu: &sync.RWMutex{},
		db: db,
	}, nil
}

func (p *sDatabase) SetLocalData(localData *models.LocalData) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	d, err := json.Marshal(localData)
	if err != nil {
		return err
	}

	return p.db.Set(keyLocalData(), d)
}

func (p *sDatabase) GetLocalData() (*models.LocalData, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	d, err := p.db.Get(keyLocalData())
	if err != nil {
		return nil, err
	}
	localData := &models.LocalData{}
	if err := json.Unmarshal(d, localData); err != nil {
		return nil, err
	}
	return localData, nil
}

func (p *sDatabase) SetClient(clientInfo *models.ClientInfo) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	pubKey := asymmetric.LoadPubKey(clientInfo.PubKey)
	if pubKey == nil {
		return errors.New("pubkey is nil")
	}
	d, err := json.Marshal(clientInfo)
	if err != nil {
		return err
	}
	return p.db.Set(keyClient(pubKey.GetHasher().ToString()), d)
}

func (p *sDatabase) GetClient(pkHash string) (*models.ClientInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	d, err := p.db.Get(keyClient(pkHash))
	if err != nil {
		return nil, err
	}
	clientInfo := &models.ClientInfo{}
	if err := json.Unmarshal(d, clientInfo); err != nil {
		return nil, err
	}
	return clientInfo, nil
}

func (p *sDatabase) SetChannel(channelInfo *models.ChannelInfo) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	d, err := json.Marshal(channelInfo)
	if err != nil {
		return err
	}

	_, err = p.db.Get(keyChannel(channelInfo.ChanID))
	if err == nil {
		return nil
	}
	if !errors.Is(err, database.ErrNotFound) {
		return err
	}

	if err := p.db.Set(keyChannel(channelInfo.ChanID), d); err != nil {
		return err
	}

	for _, participant := range channelInfo.EncList {
		pkHash := participant.PkHash
		count, err := p.getCountClientChannels(pkHash)
		if err != nil {
			return err
		}

		if err := p.db.Set(keyClientChannel(pkHash, count), []byte(channelInfo.ChanID)); err != nil {
			return err
		}

		b := encoding.Uint64ToBytes(count + 1)
		if err := p.db.Set(keyClientChannelsCount(pkHash), b[:]); err != nil {
			return err
		}
	}

	return nil
}

func (p *sDatabase) GetChannel(chanID string) (*models.ChannelInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	d, err := p.db.Get(keyChannel(chanID))
	if err != nil {
		return nil, err
	}
	channelInfo := &models.ChannelInfo{}
	if err := json.Unmarshal(d, channelInfo); err != nil {
		return nil, err
	}
	return channelInfo, nil
}

func (p *sDatabase) DelChannel(chanID string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if _, err := p.db.Get(keyChannel(chanID)); err != nil {
		return err
	}

	count, err := p.getCountChannelMessages(chanID)
	if err != nil {
		return err
	}

	for i := range count {
		msgHash, err := p.db.Get(keyChannelMessage(chanID, i))
		if err != nil {
			if errors.Is(err, database.ErrNotFound) {
				continue
			}
			return err
		}
		_ = p.db.Del(keyMessage(string(msgHash)))
		_ = p.db.Del(keyChannelMessage(chanID, i))
	}

	_ = p.db.Del(keyChannelsMessageCount(chanID))
	return p.db.Del(keyChannel(chanID))
}

func (p *sDatabase) GetCountClientChannels(pkHash string) (uint64, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.getCountClientChannels(pkHash)
}

func (p *sDatabase) GetCountChannelMessages(chanID string) (uint64, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.getCountChannelMessages(chanID)
}

func (p *sDatabase) GetClientChanIDByIndex(pkHash string, index uint64) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	count, err := p.getCountClientChannels(pkHash)
	if err != nil {
		return "", err
	}
	if index >= count {
		return "", errors.New("index > size")
	}
	chanID, err := p.db.Get(keyClientChannel(pkHash, index))
	if err != nil {
		return "", err
	}
	return string(chanID), nil
}

func (p *sDatabase) GetChannelMessageHashByIndex(chanID string, index uint64) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	count, err := p.getCountChannelMessages(chanID)
	if err != nil {
		return "", err
	}
	if index >= count {
		return "", errors.New("index > size")
	}
	msgHash, err := p.db.Get(keyChannelMessage(chanID, index))
	if err != nil {
		return "", err
	}
	return string(msgHash), nil
}

func (p *sDatabase) AddChannelMessage(messageInfo *models.MessageInfo) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	msgHash := messageInfo.GetHash()

	_, err := p.db.Get(keyMessage(msgHash))
	if err == nil {
		return nil
	}
	if !errors.Is(err, database.ErrNotFound) {
		return err
	}

	d, err := json.Marshal(messageInfo)
	if err != nil {
		return err
	}

	count, err := p.getCountChannelMessages(messageInfo.ChanID)
	if err != nil {
		return err
	}

	if err := p.db.Set(keyMessage(msgHash), d); err != nil {
		return err
	}
	if err := p.db.Set(keyChannelMessage(messageInfo.ChanID, count), []byte(msgHash)); err != nil {
		return err
	}

	b := encoding.Uint64ToBytes(count + 1)
	if err := p.db.Set(keyChannelsMessageCount(messageInfo.ChanID), b[:]); err != nil {
		return err
	}

	return nil
}

func (p *sDatabase) GetMessage(msgHash string) (*models.MessageInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	d, err := p.db.Get(keyMessage(msgHash))
	if err != nil {
		return nil, err
	}
	messageInfo := &models.MessageInfo{}
	if err := json.Unmarshal(d, messageInfo); err != nil {
		return nil, err
	}
	return messageInfo, nil
}

func (p *sDatabase) getCountClientChannels(pkHash string) (uint64, error) {
	countBytes, err := p.db.Get(keyClientChannelsCount(pkHash))
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return 0, err
	}

	count := uint64(0)
	if len(countBytes) != 0 {
		b := [encoding.CSizeUint64]byte{}
		copy(b[:], countBytes)
		count = encoding.BytesToUint64(b)
	}

	return count, nil
}

func (p *sDatabase) getCountChannelMessages(chanID string) (uint64, error) {
	countBytes, err := p.db.Get(keyChannelsMessageCount(chanID))
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return 0, err
	}

	count := uint64(0)
	if len(countBytes) != 0 {
		b := [encoding.CSizeUint64]byte{}
		copy(b[:], countBytes)
		count = encoding.BytesToUint64(b)
	}

	return count, nil
}
