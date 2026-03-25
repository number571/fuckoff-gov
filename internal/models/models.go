package models

import (
	"time"

	"github.com/number571/fuckoff-gov/internal/consts"
	"github.com/number571/fuckoff-gov/internal/strings"
	"github.com/number571/go-peer/pkg/crypto/asymmetric"
	"github.com/number571/go-peer/pkg/crypto/hashing"
	"github.com/number571/go-peer/pkg/crypto/puzzle"
)

type LocalData struct {
	NickName              string              `json:"nickname"`
	PrivKey               string              `json:"privkey"`
	Connections           map[string][]byte   `json:"connections"`
	FavoriteChannels      map[string]struct{} `json:"favorite_channels"`
	BlackListChannels     map[string]struct{} `json:"blacklist_channels"`
	BlackListParticipants map[string]struct{} `json:"blacklist_participants"`
}

type ClientInfo struct {
	PubKey string `json:"pubkey"`
	Proof  uint64 `json:"proof"`
}

func (p *ClientInfo) Validate(workSize uint64) bool {
	pubKey := asymmetric.LoadPubKey(p.PubKey)
	if pubKey == nil {
		return false
	}
	hash := pubKey.GetHasher().ToBytes()
	return puzzle.NewPoWPuzzle(workSize).VerifyBytes(hash, p.Proof)
}

type ChannelInfo struct {
	ChanID  string             `json:"chanid"`
	EncName []byte             `json:"encname"`
	EncList []*ParticipantInfo `json:"enclist"`
	Sign    []byte             `json:"sign"`
	Proof   uint64             `json:"proof"`
}

type ParticipantInfo struct {
	PkHash string `json:"pkhash"`
	Encaps []byte `json:"encaps"`
	EncKey []byte `json:"enckey"`
}

func (p *ChannelInfo) Validate(workSize uint64, pubKey asymmetric.IPubKey) bool {
	if len(p.EncList) == 0 {
		return false
	}
	if pubKey.GetHasher().ToString() != p.EncList[0].PkHash {
		return false
	}
	ok := pubKey.GetDSAPubKey().VerifyBytes([]byte(p.ChanID), p.Sign)
	if !ok {
		return false
	}
	ok = puzzle.NewPoWPuzzle(workSize).VerifyBytes([]byte(p.ChanID), p.Proof)
	if !ok {
		return false
	}
	return true
}

type MessageInfo struct {
	ChanID string `json:"chanid"`
	PkHash string `json:"pkhash"`
	EncMsg []byte `json:"encmsg"`
	Sign   []byte `json:"sign"`
	Proof  uint64 `json:"proof"`
}

type MessageBody struct {
	Filename  string    `json:"filename,omitempty"`
	Sender    string    `json:"sender"`
	Payload   []byte    `json:"payload"`
	Timestamp time.Time `json:"timestamp"`
}

func (p *MessageInfo) GetHash() string {
	return hashing.NewHMACHasher([]byte(p.ChanID), p.EncMsg).ToString()
}

func (p *MessageBody) Validate() bool {
	if strings.HasNotGraphicCharacters(p.Sender) {
		return false
	}
	if len(p.Sender) > consts.MaxNickNameSize {
		return false
	}
	if len(p.Payload) > consts.MaxMessageSize {
		return false
	}
	if p.Filename != "" {
		if strings.HasNotGraphicCharacters(p.Filename) {
			return false
		}
		if len(p.Filename) > consts.MaxFileNameSize {
			return false
		}
	}
	return true
}

func (p *MessageInfo) Validate(workSize uint64, pubKey asymmetric.IPubKey) bool {
	hash := []byte(p.GetHash())

	if p.PkHash != pubKey.GetHasher().ToString() {
		return false
	}

	if ok := pubKey.GetDSAPubKey().VerifyBytes(hash, p.Sign); !ok {
		return false
	}

	if ok := puzzle.NewPoWPuzzle(workSize).VerifyBytes(hash, p.Proof); !ok {
		return false
	}

	return true
}
