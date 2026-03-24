package clientside

import (
	"github.com/number571/fuckoff-gov/internal/database"
	"github.com/number571/fuckoff-gov/internal/models"
)

type IClientDatabase interface {
	database.ICommonDatabase

	GetLocalData() (*models.LocalData, error)
	SetLocalData(localData *models.LocalData) error
}
