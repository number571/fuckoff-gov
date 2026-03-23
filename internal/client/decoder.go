package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/number571/fuckoff-gov/internal/consts"
	"github.com/number571/fuckoff-gov/internal/crypto"
	"github.com/number571/fuckoff-gov/internal/models"
	"github.com/number571/fuckoff-gov/internal/strings"
	"github.com/number571/go-peer/pkg/crypto/asymmetric"
	"github.com/number571/go-peer/pkg/crypto/hashing"
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

	key, err := crypto.DecryptAESGCM(skey, participantInfo.EncKey)
	if err != nil {
		return nil, "", err
	}

	chanID := hashing.NewHMACHasher(key, bytes.Join(
		[][]byte{channelInfo.EncName, encList},
		[]byte{},
	)).ToString()

	if chanID != channelInfo.ChanID {
		return nil, "", errors.New("chan id is invalid")
	}

	decName, err := crypto.DecryptAESGCM(key, channelInfo.EncName) // TODO: check graphic chars
	if err != nil {
		return nil, "", err
	}
	if len(decName) > consts.MaxChannelName {
		return nil, "", fmt.Errorf("size name > max(%d)", consts.MaxChannelName)
	}
	if strings.HasNotGraphicCharacters(string(decName)) {
		return nil, "", errors.New("name has non graphical chars")
	}

	pkHashes := make([]string, 0, len(channelInfo.EncList))
	for _, v := range channelInfo.EncList {
		pkHashes = append(pkHashes, v.PkHash)
	}

	return key, string(decName), nil
}

func (p *sDecoder) MessageInfo(pubKeyCreator asymmetric.IPubKey, key []byte, messageInfo *models.MessageInfo) (*models.MessageBody, error) {
	if ok := messageInfo.Validate(p.workParams[2], pubKeyCreator); !ok {
		return nil, errors.New("invalid message")
	}
	decMsg, err := crypto.DecryptAESGCM(key, messageInfo.EncMsg)
	if err != nil {
		return nil, err
	}
	msgBody := &models.MessageBody{}
	if err := json.Unmarshal(decMsg, msgBody); err != nil {
		return nil, err
	}
	if !msgBody.Validate() {
		return nil, errors.New("invalid message body")
	}
	return msgBody, nil
}
