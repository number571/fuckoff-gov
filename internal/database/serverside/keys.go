package serverside

import "fmt"

func keyAuthSecretKey() []byte {
	return []byte("auth.secret.key")
}

func keyAuthToken(pkHash string) []byte {
	return []byte(fmt.Sprintf("auth.tokens[%s]", pkHash))
}
