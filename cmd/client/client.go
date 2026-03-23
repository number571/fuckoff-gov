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
	nn       string
	encoder  client.IEncoder
	decoder  client.IDecoder
	connects []*sConnection
	mapConns map[string]struct{}
}

func newClient(path string) *sClient {
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
			NickName: fmt.Sprintf("client-%x", random.NewRandom().GetBytes(8)),
			PrivKey:  asymmetric.NewPrivKey().ToString(),
		}
		if err := p.db.SetLocalData(ld); err != nil {
			return err
		}
	}

	p.nn = ld.NickName
	p.sk = asymmetric.LoadPrivKey(ld.PrivKey)

	p.mapConns = ld.Connections
	p.connects = p.mapConnsToList()

	works := [3]uint64{
		consts.WorkSizeClient,
		consts.WorkSizeChannel,
		consts.WorkSizeMessage,
	}

	p.encoder = client.NewEncoder(works, p.sk)
	p.decoder = client.NewDecoder(works, p.sk)

	// TODO: delete
	p.connects = append(p.connects, &sConnection{address: "http://localhost:8080"})

	return nil
}

func (p *sClient) delConnection(addr string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.mapConns[addr]; !ok {
		return nil
	}
	delete(p.mapConns, addr)

	ld := &models.LocalData{
		NickName:    p.nn,
		PrivKey:     p.sk.ToString(),
		Connections: p.mapConns,
	}

	if err := gClient.db.SetLocalData(ld); err != nil {
		return err
	}

	p.connects = p.mapConnsToList()
	return nil
}

func (p *sClient) addConnection(addr string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.mapConns[addr]; ok {
		return nil
	}

	ld := &models.LocalData{
		NickName:    p.nn,
		PrivKey:     p.sk.ToString(),
		Connections: p.mapConns,
	}

	if err := gClient.db.SetLocalData(ld); err != nil {
		return err
	}

	p.mapConns[addr] = struct{}{}
	p.connects = p.mapConnsToList()

	return nil
}

func (p *sClient) inConnections(c *sConnection) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	_, ok := p.mapConns[c.address]
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

	ld := &models.LocalData{
		NickName:    nn,
		PrivKey:     p.sk.ToString(),
		Connections: p.mapConns,
	}

	if err := gClient.db.SetLocalData(ld); err != nil {
		return err
	}
	p.nn = nn

	return nil
}

func (p *sClient) getNickName() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.nn
}

func (p *sClient) mapConnsToList() []*sConnection {
	connects := make([]*sConnection, 0, len(p.mapConns))
	for k := range p.mapConns {
		connects = append(connects, &sConnection{address: k})
	}
	slices.SortFunc(connects, func(v1, v2 *sConnection) int {
		return strings.Compare(v1.address, v2.address)
	})
	return connects
}
