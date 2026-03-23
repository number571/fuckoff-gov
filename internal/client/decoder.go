package client

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/number571/fuckoff-gov/internal/models"
	"github.com/number571/go-peer/pkg/crypto/asymmetric"
	"github.com/number571/go-peer/pkg/crypto/hashing"
	"github.com/number571/go-peer/pkg/crypto/symmetric"
)

type sDecoder struct {
	workParams [3]uint64
	privKey    asymmetric.IPrivKey
}

func NewDecoder(workParams [3]uint64, privKey asymmetric.IPrivKey) IDecoder {
	return &sDecoder{
		workParams: workParams,
		privKey:    privKey,
	}
}

func (p *sDecoder) ClientInfo(clientInfo *models.ClientInfo) (asymmetric.IPubKey, error) {
	if ok := clientInfo.Validate(p.workParams[0]); !ok {
		return nil, errors.New("invalid client")
	}
	pubKey := asymmetric.LoadPubKey(clientInfo.PubKey)
	if pubKey == nil {
		return nil, errors.New("invalid pubkey")
	}
	return pubKey, nil
}

func (p *sDecoder) ChannelInfo(pubKeyCreator asymmetric.IPubKey, channelInfo *models.ChannelInfo) ([]byte, string, error) {
	pk := p.privKey.GetPubKey()
	pkhash := pk.GetHasher().ToString()

	if len(channelInfo.EncList) == 0 {
		return nil, "", errors.New("enc list = 0")
	}
	if pubKeyCreator.GetHasher().ToString() != channelInfo.EncList[0].PkHash {
		return nil, "", errors.New("invalid pk hash creator")
	}
	if ok := channelInfo.Validate(p.workParams[1], pubKeyCreator); !ok {
		return nil, "", errors.New("invalid channel")
	}

	var participantInfo *models.ParticipantInfo
	for _, v := range channelInfo.EncList {
		if v.PkHash == pkhash {
			participantInfo = v
			break
		}
	}
	if participantInfo == nil {
		return nil, "", errors.New("pk not in list")
	}

	skey, err := p.privKey.GetKEMPrivKey().Decapsulate(participantInfo.Encaps)
	if err != nil {
		return nil, "", err
	}

	encList, err := json.Marshal(channelInfo.EncList)
	if err != nil {
		return nil, "", err
	}

	key := symmetric.NewCipher(skey).DecryptBytes(participantInfo.EncKey)
	chanID := hashing.NewHMACHasher(key, bytes.Join(
		[][]byte{channelInfo.EncName, encList},
		[]byte{},
	)).ToString()

	if chanID != channelInfo.ChanID {
		return nil, "", errors.New("chan id is invalid")
	}

	name := symmetric.NewCipher(key).DecryptBytes(channelInfo.EncName) // TODO: check graphic chars
	pkHashes := make([]string, 0, len(channelInfo.EncList))
	for _, v := range channelInfo.EncList {
		pkHashes = append(pkHashes, v.PkHash)
	}

	return key, string(name), nil
}

func (p *sDecoder) MessageInfo(pubKeyCreator asymmetric.IPubKey, key []byte, messageInfo *models.MessageInfo) (*models.MessageBody, error) {
	if ok := messageInfo.Validate(p.workParams[2], pubKeyCreator); !ok {
		return nil, errors.New("invalid message")
	}
	decMsg := symmetric.NewCipher(key).DecryptBytes(messageInfo.EncMsg)
	msgBody := &models.MessageBody{}
	if err := json.Unmarshal(decMsg, msgBody); err != nil {
		return nil, err
	}
	// TODO: check name graphic chars
	return msgBody, nil
}
