package main

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/number571/fuckoff-gov/internal/client"
	"github.com/number571/fuckoff-gov/internal/consts"
	"github.com/number571/fuckoff-gov/internal/database"
	"github.com/number571/fuckoff-gov/internal/models"
	"github.com/number571/go-peer/pkg/crypto/asymmetric"
	"github.com/number571/go-peer/pkg/crypto/random"
	gp_database "github.com/number571/go-peer/pkg/storage/database"
)

type sClient struct {
	mu       *sync.RWMutex
	path     string
	db       database.IDatabase
	sk       asymmetric.IPrivKey
	encoder  client.IEncoder
	decoder  client.IDecoder
	connects []*sConnection
	channels []*sChannel
	ld       *models.LocalData
}

func newLocalDataClient(path string) *sClient {
	return &sClient{
		mu:   &sync.RWMutex{},
		path: path,
	}
}

func (p *sClient) init() error {
	var err error

	p.db, err = database.OpenDatabase(p.path)
	if err != nil {
		return err
	}

	ld, err := p.db.GetLocalData()
	if err != nil {
		if !errors.Is(err, gp_database.ErrNotFound) {
			return err
		}
		ld = &models.LocalData{
			NickName:              fmt.Sprintf("client-%x", random.NewRandom().GetBytes(8)),
			PrivKey:               asymmetric.NewPrivKey().ToString(),
			Connections:           make(map[string]struct{}),
			FavoriteChannels:      make(map[string]struct{}),
			BlackListChannels:     make(map[string]struct{}),
			BlackListParticipants: make(map[string]struct{}),
		}
		if err := p.db.SetLocalData(ld); err != nil {
			return err
		}
	}

	p.ld = ld

	p.connects = p.mapConnsToList()
	p.sk = asymmetric.LoadPrivKey(ld.PrivKey)

	works := [3]uint64{
		consts.WorkSizeClient,
		consts.WorkSizeChannel,
		consts.WorkSizeMessage,
	}

	p.encoder = client.NewEncoder(works, p.sk)
	p.decoder = client.NewDecoder(works, p.sk)

	// TODO: delete
	p.ld.Connections["http://localhost:8080"] = struct{}{}
	p.connects = append(p.connects, &sConnection{address: "http://localhost:8080"})

	return nil
}

func (p *sClient) delConnection(addr string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.ld.Connections[addr]; !ok {
		return nil
	}
	delete(p.ld.Connections, addr)

	if err := gClient.db.SetLocalData(p.ld); err != nil {
		return err
	}

	p.connects = p.mapConnsToList()
	return nil
}

func (p *sClient) addConnection(addr string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.ld.Connections[addr]; ok {
		return nil
	}

	if err := gClient.db.SetLocalData(p.ld); err != nil {
		return err
	}

	p.ld.Connections[addr] = struct{}{}
	p.connects = p.mapConnsToList()

	return nil
}

func (p *sClient) inConnections(c *sConnection) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	_, ok := p.ld.Connections[c.address]
	return ok
}

func (p *sClient) getConnections() []*sConnection {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.connects
}

func (p *sClient) setNickName(nn string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.ld.NickName = nn
	if err := gClient.db.SetLocalData(p.ld); err != nil {
		return err
	}

	return nil
}

func (p *sClient) getNickName() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.ld.NickName
}

func (p *sClient) mapConnsToList() []*sConnection {
	connects := make([]*sConnection, 0, len(p.ld.Connections))
	for k := range p.ld.Connections {
		connects = append(connects, &sConnection{address: k})
	}
	slices.SortFunc(connects, func(v1, v2 *sConnection) int {
		return strings.Compare(v1.address, v2.address)
	})
	return connects
}

func (p *sClient) isBlockedChannel(chanID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	_, ok := p.ld.BlackListChannels[chanID]
	return ok
}

func (p *sClient) setBlockedChannel(chanID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.ld.BlackListChannels[chanID] = struct{}{}
	if err := gClient.db.SetLocalData(p.ld); err != nil {
		return err
	}

	return nil
}

func (p *sClient) isFavoriteChannel(chanID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	_, ok := p.ld.FavoriteChannels[chanID]
	return ok
}

func (p *sClient) setFavoriteChannel(chanID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.ld.FavoriteChannels[chanID] = struct{}{}
	if err := gClient.db.SetLocalData(p.ld); err != nil {
		return err
	}

	return nil
}

func (p *sClient) delFavoriteChannel(chanID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.ld.FavoriteChannels, chanID)
	if err := gClient.db.SetLocalData(p.ld); err != nil {
		return err
	}

	return nil
}
