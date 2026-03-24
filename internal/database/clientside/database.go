package clientside

import (
	"encoding/json"
	"sync"

	"github.com/number571/fuckoff-gov/internal/database"
	"github.com/number571/fuckoff-gov/internal/models"
	gp_database "github.com/number571/go-peer/pkg/storage/database"
)

type sDatabase struct {
	mu *sync.RWMutex
	db gp_database.IKVDatabase
	database.ICommonDatabase
}

func OpenClientDatabase(path string) (IClientDatabase, error) {
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
