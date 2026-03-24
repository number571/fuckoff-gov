package serverside

import (
	"errors"
	"sync"

	"github.com/number571/fuckoff-gov/internal/database"
	"github.com/number571/go-peer/pkg/crypto/random"
	gp_database "github.com/number571/go-peer/pkg/storage/database"
)

type sDatabase struct {
	mu *sync.RWMutex
	db gp_database.IKVDatabase
	database.ICommonDatabase
}

func OpenServerDatabase(path string) (IServerDatabase, error) {
	db, err := gp_database.NewKVDatabase(path)
	if err != nil {
		return nil, err
	}
	return &sDatabase{
		mu:              &sync.RWMutex{},
		db:              db,
		ICommonDatabase: database.NewCommonDatabase(db),
	}, nil
}

func (p *sDatabase) GetSecretKey() ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	secretKey, err := p.db.Get(keyAuthSecretKey())
	if err == nil {
		return secretKey, nil
	}
	if !errors.Is(err, gp_database.ErrNotFound) {
		return nil, err
	}

	secretKey = random.NewRandom().GetBytes(32)
	if err := p.db.Set(keyAuthSecretKey(), secretKey); err != nil {
		return nil, err
	}

	return secretKey, nil
}

func (p *sDatabase) SetAuthToken(pkHash, authToken string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.db.Set(keyAuthToken(pkHash), []byte(authToken))
}

func (p *sDatabase) GetAuthToken(pkHash string) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	authToken, err := p.db.Get(keyAuthToken(pkHash))
	if err != nil {
		return "", err
	}

	return string(authToken), nil
}
