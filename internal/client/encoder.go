package client

import (
	"bytes"
	"encoding/json"

	"github.com/number571/fuckoff-gov/internal/crypto"
	"github.com/number571/fuckoff-gov/internal/models"
	"github.com/number571/go-peer/pkg/crypto/asymmetric"
	"github.com/number571/go-peer/pkg/crypto/hashing"
	"github.com/number571/go-peer/pkg/crypto/puzzle"
	"github.com/number571/go-peer/pkg/crypto/random"
	"github.com/number571/go-peer/pkg/crypto/symmetric"
)

type sEncoder struct {
	workParams [3]uint64
	privKey    asymmetric.IPrivKey
}

func NewEncoder(workParams [3]uint64, privKey asymmetric.IPrivKey) IEncoder {
	return &sEncoder{
		workParams: workParams,
		privKey:    privKey,
	}
}

func (p *sEncoder) InitClient() *models.ClientInfo {
	pubKey := p.privKey.GetPubKey()

	hash := pubKey.GetHasher().ToBytes()
	proof := puzzle.NewPoWPuzzle(p.workParams[0]).ProofBytes(hash, 32)

	return &models.ClientInfo{
		PubKey: pubKey.ToString(),
		Proof:  proof,
	}
}

func (p *sEncoder) InitChannel(name string, pubKeys []asymmetric.IPubKey) *models.ChannelInfo {
	// TODO: check graphic chars in name

	key := random.NewRandom().GetBytes(symmetric.CCipherKeySize)

	list := make([]*models.ParticipantInfo, 0, len(pubKeys)+1)

	pk := p.privKey.GetPubKey()
	ct, sk, err := pk.GetKEMPubKey().Encapsulate()
	if err != nil {
		panic(err)
	}

	encKey, err := crypto.EncryptAESGCM(sk, key)
	if err != nil {
		panic(err)
	}

	list = append(list, &models.ParticipantInfo{
		PkHash: pk.GetHasher().ToString(),
		Encaps: ct,
		EncKey: encKey,
	})

	for _, pk := range pubKeys {
		ct, sk, err := pk.GetKEMPubKey().Encapsulate()
		if err != nil {
			panic(err)
		}
		encKey, err := crypto.EncryptAESGCM(sk, key)
		if err != nil {
			panic(err)
		}
		list = append(list, &models.ParticipantInfo{
			PkHash: pk.GetHasher().ToString(),
			Encaps: ct,
			EncKey: encKey,
		})
	}

	encList, err := json.Marshal(list)
	if err != nil {
		panic(err)
	}

	encName, err := crypto.EncryptAESGCM(key, []byte(name))
	if err != nil {
		panic(err)
	}

	chanID := hashing.NewHMACHasher(key, bytes.Join(
		[][]byte{encName, encList},
		[]byte{},
	)).ToString()

	return &models.ChannelInfo{
		ChanID:  chanID,
		EncName: encName,
		EncList: list,
		Sign:    p.privKey.GetDSAPrivKey().SignBytes([]byte(chanID)),
		Proof:   puzzle.NewPoWPuzzle(p.workParams[1]).ProofBytes([]byte(chanID), 64),
	}
}

func (p *sEncoder) PushMessage(chanID string, key []byte, msgBody *models.MessageBody) *models.MessageInfo {
	bodyBytes, err := json.Marshal(msgBody)
	if err != nil {
		panic(err)
	}
	encMsg, err := crypto.EncryptAESGCM(key, bodyBytes)
	if err != nil {
		panic(err)
	}
	hashMessage := hashing.NewHMACHasher([]byte(chanID), encMsg).ToString()
	return &models.MessageInfo{
		ChanID: chanID,
		PkHash: p.privKey.GetPubKey().GetHasher().ToString(),
		EncMsg: encMsg,
		Sign:   p.privKey.GetDSAPrivKey().SignBytes([]byte(hashMessage)),
		Proof:  puzzle.NewPoWPuzzle(p.workParams[2]).ProofBytes([]byte(hashMessage), 64),
	}
}
