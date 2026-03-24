package serverside

import (
	"github.com/number571/fuckoff-gov/internal/database"
)

type IServerDatabase interface {
	database.ICommonDatabase

	GetSecretKey() ([]byte, error)

	GetAuthToken(pkHash string) (string, error)
	SetAuthToken(pkHash, authToken string) error
}
