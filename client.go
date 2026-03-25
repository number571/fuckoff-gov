package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/number571/fuckoff-gov/internal/client"
	"github.com/number571/fuckoff-gov/internal/consts"
	"github.com/number571/fuckoff-gov/internal/database/clientside"
	"github.com/number571/fuckoff-gov/internal/models"
	"github.com/number571/go-peer/pkg/crypto/asymmetric"
	"github.com/number571/go-peer/pkg/crypto/hashing"
	"github.com/number571/go-peer/pkg/crypto/random"
	gp_database "github.com/number571/go-peer/pkg/storage/database"
)

type sClient struct {
	muInit   *sync.Mutex
	mu       *sync.RWMutex
	path     string
	db       clientside.IClientDatabase
	sk       asymmetric.IPrivKey
	encoder  client.IEncoder
	decoder  client.IDecoder
	connects []*sConnection
	channels *sChannelsList
	ld       *models.LocalData
}

func newLocalDataClient(path string) *sClient {
	return &sClient{
		muInit:   &sync.Mutex{},
		mu:       &sync.RWMutex{},
		path:     path,
		channels: newChannelsList(),
	}
}

func (p *sClient) init() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var err error

	p.db, err = clientside.OpenClientDatabase(p.path)
	if err != nil {
		return err
	}

	p.ld, err = p.db.GetLocalData()
	if err != nil {
		if !errors.Is(err, gp_database.ErrNotFound) {
			return err
		}
		p.ld = &models.LocalData{
			NickName:              fmt.Sprintf("client-%x", random.NewRandom().GetBytes(8)),
			PrivKey:               asymmetric.NewPrivKey().ToString(),
			Connections:           make(map[string][]byte),
			FavoriteChannels:      make(map[string]struct{}),
			BlackListChannels:     make(map[string]struct{}),
			BlackListParticipants: make(map[string]struct{}),
		}
		if err := p.db.SetLocalData(p.ld); err != nil {
			return err
		}
	}

	p.sk = asymmetric.LoadPrivKey(p.ld.PrivKey)
	p.mapConnectsToList()

	works := [3]uint64{
		consts.WorkSizeClient,
		consts.WorkSizeChannel,
		consts.WorkSizeMessage,
	}

	p.encoder = client.NewEncoder(works, p.sk)
	p.decoder = client.NewDecoder(works, p.sk)

	return nil
}

func (p *sClient) addConnection(cert *x509.Certificate) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	certID := getCertID(cert)
	if _, ok := p.ld.Connections[certID]; ok {
		return certID, nil
	}
	p.ld.Connections[certID] = certToBytes(cert)

	if err := gClient.db.SetLocalData(p.ld); err != nil {
		return "", err
	}

	p.mapConnectsToList()
	return certID, nil
}

func (p *sClient) delConnection(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.ld.Connections[id]; !ok {
		return nil
	}
	delete(p.ld.Connections, id)

	if err := gClient.db.SetLocalData(p.ld); err != nil {
		return err
	}

	p.mapConnectsToList()
	return nil
}

func (p *sClient) inConnections(id string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	_, ok := p.ld.Connections[id]
	return ok
}

func (p *sClient) getConnectionByID(id string) (*sConnection, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, c := range p.connects {
		if c.id == id {
			return c, true
		}
	}
	return nil, false
}

func (p *sClient) getConnections() []*sConnection {
	p.mu.RLock()
	defer p.mu.RUnlock()

	connects := make([]*sConnection, len(p.connects))
	copy(connects, p.connects)
	return connects
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

func (p *sClient) mapConnectsToList() {
	currentConnectsMap := make(map[string]*sConnection, len(p.connects))
	for _, v := range p.connects {
		currentConnectsMap[v.id] = v
	}

	resultConnectsList := make([]*sConnection, 0, len(p.ld.Connections))
	for id, certBytes := range p.ld.Connections {
		v, ok := currentConnectsMap[id]
		if ok {
			resultConnectsList = append(resultConnectsList, v)
			continue
		}
		cert, err := bytesToCert(certBytes)
		if err != nil {
			panic(err)
		}
		resultConnectsList = append(
			resultConnectsList,
			&sConnection{
				id:     id,
				cert:   cert,
				client: newConn(cert, p.sk),
			},
		)
	}

	slices.SortFunc(resultConnectsList, func(v1, v2 *sConnection) int {
		return strings.Compare(v1.id, v2.id)
	})

	p.connects = resultConnectsList
}

func (p *sClient) isBlockedParticipant(pkHash string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	_, ok := p.ld.BlackListParticipants[pkHash]
	return ok
}

func (p *sClient) setBlockedParticipant(pkHash string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.ld.BlackListParticipants[pkHash] = struct{}{}

	if err := gClient.db.SetLocalData(p.ld); err != nil {
		return err
	}

	return nil
}

func (p *sClient) unsetBlockedParticipant(pkHash string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.ld.BlackListParticipants[pkHash]; !ok {
		return nil
	}
	delete(p.ld.BlackListParticipants, pkHash)

	if err := gClient.db.SetLocalData(p.ld); err != nil {
		return err
	}

	return nil
}

func (p *sClient) isDeletedChannel(chanID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	_, ok := p.ld.BlackListChannels[chanID]
	return ok
}

func (p *sClient) setDeletedChannel(chanID string) error {
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

func (p *sClient) unsetFavoriteChannel(chanID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.ld.FavoriteChannels[chanID]; !ok {
		return nil
	}
	delete(p.ld.FavoriteChannels, chanID)

	if err := gClient.db.SetLocalData(p.ld); err != nil {
		return err
	}

	return nil
}

func bytesToCert(certBytes []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(certBytes)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("invalid certificate block")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	if len(cert.IPAddresses) == 0 && len(cert.DNSNames) == 0 {
		return nil, errors.New("undefined host")
	}
	if len(cert.Subject.Organization) == 0 {
		return nil, errors.New("undefined port")
	}
	if _, err := strconv.Atoi(cert.Subject.Organization[0]); err != nil {
		return nil, errors.New("invalid port")
	}
	return cert, nil
}

func certToBytes(cert *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
}

func getCertID(cert *x509.Certificate) string {
	return hashing.NewHasher(cert.Raw).ToString()
}

func newConn(cert *x509.Certificate, privKey asymmetric.IPrivKey) client.IClient {
	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(certToBytes(cert)); !ok {
		panic("Failed to append CA cert to pool")
	}
	return client.NewClient(
		fmt.Sprintf("https://%s:%s", getAddrFromCert(cert), cert.Subject.Organization[0]),
		privKey,
		&http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs: caCertPool,
				},
			},
		},
	)
}
