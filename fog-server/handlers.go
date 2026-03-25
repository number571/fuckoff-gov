package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/number571/fuckoff-gov/internal/consts"
	"github.com/number571/fuckoff-gov/internal/models"
	"github.com/number571/go-peer/pkg/crypto/asymmetric"
	"github.com/number571/go-peer/pkg/crypto/random"
	"github.com/number571/go-peer/pkg/storage/database"
)

var (
	mapAuthTasksMtx = &sync.Mutex{}
	mapAuthTasks    = make(map[string]string)
)

func handlePing(w http.ResponseWriter, r *http.Request) {}

func handleAuth(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10) // 16KiB

	queryParams := r.URL.Query()
	pkHash := queryParams.Get("pkhash")

	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	clientInfo, err := db.GetClient(pkHash)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	if r.Method == http.MethodGet {
		mapAuthTasksMtx.Lock()
		defer mapAuthTasksMtx.Unlock()

		authTask := random.NewRandom().GetString(96)
		mapAuthTasks[pkHash] = authTask

		w.Header().Set(consts.HeaderAuthTask, authTask)
		return
	}

	mapAuthTasksMtx.Lock()
	authTask, ok := mapAuthTasks[pkHash]
	delete(mapAuthTasks, pkHash)
	mapAuthTasksMtx.Unlock()

	if !ok {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	sign, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	pubKey := asymmetric.LoadPubKey(clientInfo.PubKey)
	if ok := pubKey.GetDSAPubKey().VerifyBytes([]byte(authTask), sign); !ok {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	jwtToken, err := createToken(pkHash)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := db.SetAuthToken(pkHash, jwtToken); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set(consts.HeaderAuthToken, jwtToken)
}

func handleClientInit(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10) // 16KiB

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	clientInfo := &models.ClientInfo{}
	if err := json.NewDecoder(r.Body).Decode(clientInfo); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if ok := clientInfo.Validate(consts.WorkSizeClient); !ok {
		w.WriteHeader(http.StatusTeapot)
		return
	}

	pubKey := asymmetric.LoadPubKey(clientInfo.PubKey)
	if _, err := db.GetClient(pubKey.GetHasher().ToString()); err == nil {
		// Client already exist
		return
	}

	if err := db.SetClient(clientInfo); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func handleClientLoad(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	queryParams := r.URL.Query()
	hash := queryParams.Get("pkhash")

	clientInfo, err := db.GetClient(hash)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	if err := json.NewEncoder(w).Encode(clientInfo); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func handleClientChannelsSize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	pkHash, err := verifyToken(r.Header.Get(consts.HeaderAuthToken))
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	count, err := db.GetCountClientChannels(pkHash)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "%d", count)
}

func handleClientChannelsListen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	queryParams := r.URL.Query()

	pkHash, err := verifyToken(r.Header.Get(consts.HeaderAuthToken))
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	index, err := strconv.ParseUint(queryParams.Get("index"), 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancel()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		count, err := db.GetCountClientChannels(pkHash)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if count > index {
			chanID, err := db.GetClientChanIDByIndex(pkHash, index)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			fmt.Fprint(w, chanID)
			return
		}
		select {
		case <-ctx.Done():
			w.WriteHeader(http.StatusNoContent)
			return
		case <-ticker.C:
		}
	}
}

func participantInChannel(channelInfo *models.ChannelInfo, pkHash string) bool {
	for _, v := range channelInfo.EncList {
		if v.PkHash == pkHash {
			return true
		}
	}
	return false
}

func handleChannelInit(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20) // 2MiB

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	pkHash, err := verifyToken(r.Header.Get(consts.HeaderAuthToken))
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	channelInfo := &models.ChannelInfo{}
	if err := json.NewDecoder(r.Body).Decode(channelInfo); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !participantInChannel(channelInfo, pkHash) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// get channel's author
	clientInfo, err := db.GetClient(channelInfo.EncList[0].PkHash)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	pubKey := asymmetric.LoadPubKey(clientInfo.PubKey)
	if ok := channelInfo.Validate(consts.WorkSizeChannel, pubKey); !ok {
		w.WriteHeader(http.StatusTeapot)
		return
	}

	if _, err := db.GetChannel(channelInfo.ChanID); err == nil {
		// channel already exist
		return
	}

	if err := db.SetChannel(channelInfo); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func handleChannelLoad(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	pkHash, err := verifyToken(r.Header.Get(consts.HeaderAuthToken))
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	queryParams := r.URL.Query()
	chanID := queryParams.Get("chanid")

	channelInfo, err := db.GetChannel(chanID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	if !participantInChannel(channelInfo, pkHash) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if err := json.NewEncoder(w).Encode(channelInfo); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func handleChannelChatSize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	pkHash, err := verifyToken(r.Header.Get(consts.HeaderAuthToken))
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	queryParams := r.URL.Query()
	chanID := queryParams.Get("chanid")

	channelInfo, err := db.GetChannel(chanID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	if !participantInChannel(channelInfo, pkHash) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	count, err := db.GetCountChannelMessages(chanID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "%d", count)
}

func handleChannelChatPush(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20) // 2MiB

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	pkHash, err := verifyToken(r.Header.Get(consts.HeaderAuthToken))
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	messageInfo := &models.MessageInfo{}
	if err := json.NewDecoder(r.Body).Decode(messageInfo); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	channelInfo, err := db.GetChannel(messageInfo.ChanID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	if !participantInChannel(channelInfo, pkHash) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	clientInfo, err := db.GetClient(messageInfo.PkHash)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	pubKey := asymmetric.LoadPubKey(clientInfo.PubKey)
	if pubKey == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if ok := messageInfo.Validate(consts.WorkSizeMessage, pubKey); !ok {
		w.WriteHeader(http.StatusTeapot)
		return
	}

	if err := db.AddChannelMessage(messageInfo); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func handleChannelChatLoad(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	pkHash, err := verifyToken(r.Header.Get(consts.HeaderAuthToken))
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	queryParams := r.URL.Query()
	messageHash := queryParams.Get("hash")

	messageInfo, err := db.GetMessage(messageHash)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	channelInfo, err := db.GetChannel(messageInfo.ChanID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	if !participantInChannel(channelInfo, pkHash) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if err := json.NewEncoder(w).Encode(messageInfo); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func handleChannelChatListen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	pkHash, err := verifyToken(r.Header.Get(consts.HeaderAuthToken))
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	queryParams := r.URL.Query()
	chanID := queryParams.Get("chanid")

	channelInfo, err := db.GetChannel(chanID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	if !participantInChannel(channelInfo, pkHash) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	index, err := strconv.ParseUint(queryParams.Get("index"), 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancel()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		count, err := db.GetCountChannelMessages(chanID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if count > index {
			msgHash, err := db.GetChannelMessageHashByIndex(chanID, index)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			fmt.Fprint(w, msgHash)
			return
		}
		select {
		case <-ctx.Done():
			w.WriteHeader(http.StatusNoContent)
			return
		case <-ticker.C:
		}
	}
}
