package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"
	"github.com/number571/fuckoff-gov/internal/client"
	"github.com/number571/fuckoff-gov/internal/consts"
	"github.com/number571/fuckoff-gov/internal/models"
	"github.com/number571/go-peer/pkg/crypto/asymmetric"

	gp_database "github.com/number571/go-peer/pkg/storage/database"
)

var (
	gClient   = newLocalDataClient(os.Args[1])
	gChannels = newChannelsList()
)

var (
	aboutPageContainer    = new(fyne.Container)
	addChannelsContainer  = new(fyne.Container)
	listChannelsContainer = new(fyne.Container)
	chatChannelContainer  = new(fyne.Container)
	chatSettingsContainer = new(fyne.Container)
	chatSearchContainer   = new(fyne.Container)
	connectionsContainer  = new(fyne.Container)
)

func init() {
	if err := gClient.init(); err != nil {
		panic(err)
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a := app.NewWithID("fuckoff.gov.chat")

	w := a.NewWindow("Fuckoff Gov Chat")
	w.Resize(fyne.NewSize(600, 400))

	aboutPageContainer = initWindowAboutPage(ctx, a, w)
	addChannelsContainer = initWindowAddChannels(ctx, a, w)
	listChannelsContainer = initWindowListChannels(ctx, a, w)
	chatChannelContainer = initWindowChatChannel(ctx, a, w)
	chatSettingsContainer = initWindowChatSettings(ctx, a, w)
	chatSearchContainer = initWindowChatSearch(ctx, a, w)
	connectionsContainer = initWindowConnections(ctx, a, w)

	go runClientInitializer(ctx, w)
	go runChannelsListener(ctx, w)

	setChatListContent(w)
	w.ShowAndRun()
}

func runClientInitializer(ctx context.Context, w fyne.Window) {
	clientInfo, err := initLocalClient()
	if err != nil {
		fyne.Do(func() { dialog.ShowError(err, w) })
		return
	}
	initRemoteClient(ctx, clientInfo)
}

func runChannelsListener(ctx context.Context, w fyne.Window) {
	pkHash := gClient.sk.GetPubKey().GetHasher().ToString()

	counter, err := gClient.db.GetCountClientChannels(pkHash)
	if err != nil {
		fyne.Do(func() { dialog.ShowError(err, w) })
		return
	}

	for i := range counter {
		chanID, err := gClient.db.GetClientChanIDByIndex(pkHash, i)
		if err != nil {
			fyne.Do(func() { dialog.ShowError(err, w) })
			return
		}
		channelInfo, err := gClient.db.GetChannel(chanID)
		if err != nil {
			fyne.Do(func() { dialog.ShowError(err, w) })
			return
		}
		if err := addChannelIntoList(ctx, channelInfo); err != nil {
			fyne.Do(func() { dialog.ShowError(err, w) })
			return
		}
		if err := initRemoteChannel(ctx, channelInfo); err != nil {
			fyne.Do(func() { printLog(logErro, err) })
			continue
		}
	}

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	connsMapper := make(map[string]struct{})
	for {
		for _, c := range gClient.getConnections() {
			if _, ok := connsMapper[c.address]; ok {
				continue
			}
			connsMapper[c.address] = struct{}{}
			go runChannelsListenerOnConnection(ctx, c, pkHash)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func runMessagesListener(ctx context.Context, w fyne.Window, channel *sChannel) {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		chatListenerActive = true
		<-closeListenChat
		chatListenerActive = false
		cancel()
	}()

	counter, err := gClient.db.GetCountChannelMessages(channel.chanID)
	if err != nil {
		fyne.Do(func() { dialog.ShowError(err, w) })
		return
	}

	index := uint64(0)
	if counter > consts.CountMessagesPerPage {
		index = counter - consts.CountMessagesPerPage
	}

	if index == 0 {
		startChatIndexReader = 0
	} else {
		startChatIndexReader = index - 1
	}

	for index < counter {
		msgHash, err := gClient.db.GetChannelMessageHashByIndex(channel.chanID, index)
		if err != nil {
			fyne.Do(func() { dialog.ShowError(err, w) })
			return
		}
		messageInfo, err := gClient.db.GetMessage(msgHash)
		if err != nil {
			fyne.Do(func() { dialog.ShowError(err, w) })
			return
		}
		pubKey, ok := channel.pubKeysMap[messageInfo.PkHash]
		if !ok {
			fyne.Do(func() { dialog.ShowError(err, w) })
			return
		}
		msgBody, err := gClient.decoder.MessageInfo(pubKey, channel.key, messageInfo)
		if err != nil {
			fyne.Do(func() { dialog.ShowError(err, w) })
			return
		}
		index++
		fyne.Do(func() { addMessageToChat(w, scrollChatContainer, pubKey, msgBody, false) })
	}

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	connsMapper := make(map[string]struct{})
	for {
		for _, c := range gClient.getConnections() {
			if _, ok := connsMapper[c.address]; ok {
				continue
			}
			connsMapper[c.address] = struct{}{}
			go runMessagesListenerOnConnection(ctx, w, channel, c)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func runMessagesListenerOnConnection(ctx context.Context, w fyne.Window, channel *sChannel, c *sConnection) {
	appClient := newConn(c.address)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if !gClient.inConnections(c) {
			return
		}

		sizeChan, err := appClient.CountMessages(ctx, channel.chanID)
		if err != nil {
			fyne.Do(func() { printLog(logErro, err) })
			timeSleep(ctx, time.Second)
			continue
		}

		counter, err := binarySearchCounter(ctx, channel, appClient, int64(sizeChan)-1)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				fyne.Do(func() { printLog(logErro, err) })
			}
			timeSleep(ctx, time.Second)
			continue
		}

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			messageInfo, err := appClient.ListenMessage(ctx, channel.chanID, counter)
			if err != nil {
				if !errors.Is(err, context.Canceled) {
					fyne.Do(func() { printLog(logErro, err) })
				}
				timeSleep(ctx, time.Second)
				continue
			}
			msgHash := messageInfo.GetHash()
			if _, err = gClient.db.GetMessage(msgHash); err == nil {
				counter++
				continue
			}
			pubKey, ok := channel.pubKeysMap[messageInfo.PkHash]
			if !ok {
				fyne.Do(func() { printLog(logErro, errors.New("pubkey not found")) })
				timeSleep(ctx, time.Second)
				continue
			}
			counter++
			msgBody, err := gClient.decoder.MessageInfo(pubKey, channel.key, messageInfo)
			if err != nil {
				fyne.Do(func() { printLog(logErro, err) })
				timeSleep(ctx, time.Second)
				continue
			}
			if err := gClient.db.AddChannelMessage(messageInfo); err != nil {
				fyne.Do(func() { printLog(logErro, err) })
				timeSleep(ctx, time.Second)
				continue
			}
			if err := pushRemoteMessage(ctx, messageInfo); err != nil {
				fyne.Do(func() { printLog(logErro, err) })
				timeSleep(ctx, time.Second)
				continue
			}
			fyne.Do(func() { addMessageToChat(w, scrollChatContainer, pubKey, msgBody, false) })
		}
	}
}

func runChannelsListenerOnConnection(ctx context.Context, c *sConnection, pkHash string) {
	appClient := newConn(c.address)
	counter := uint64(0)

	for {
		if !gClient.inConnections(c) {
			return
		}

		channelInfo, err := appClient.ListenChannel(ctx, pkHash, counter)
		if err != nil {
			fyne.Do(func() { printLog(logErro, err) })
			timeSleep(ctx, time.Second)
			continue
		}

		_, err = gClient.db.GetChannel(channelInfo.ChanID)
		if err == nil {
			counter++
			continue
		}
		if !errors.Is(err, gp_database.ErrNotFound) {
			fyne.Do(func() { printLog(logErro, err) })
			timeSleep(ctx, time.Second)
			continue
		}

		if err := addChannelIntoList(ctx, channelInfo); err != nil {
			fyne.Do(func() { printLog(logErro, err) })
			timeSleep(ctx, time.Second)
			continue
		}

		if err := gClient.db.SetChannel(channelInfo); err != nil {
			fyne.Do(func() { printLog(logErro, err) })
			timeSleep(ctx, time.Second)
			continue
		}

		counter++
	}
}

func initRemoteChannel(ctx context.Context, channelInfo *models.ChannelInfo) error {
	var (
		lastErr    error
		hasSuccess bool
	)
	for _, c := range gClient.getConnections() {
		if err := newConn(c.address).InitChannel(ctx, channelInfo); err != nil {
			lastErr = err
			continue
		}
		hasSuccess = true
	}
	if hasSuccess {
		return nil
	}
	return lastErr
}

func initLocalChannel(ctx context.Context, chanName string, pkHashes []string) (*models.ChannelInfo, error) {
	pubKeys := make([]asymmetric.IPubKey, 0, len(pkHashes))
	for _, pkHash := range pkHashes {
		pubKey, _, err := getClientInfo(ctx, pkHash)
		if err != nil {
			return nil, err
		}
		pubKeys = append(pubKeys, pubKey)
	}
	channelInfo, err := gClient.encoder.InitChannel(chanName, pubKeys)
	if err != nil {
		return nil, err
	}
	if err := addChannelIntoList(ctx, channelInfo); err != nil {
		return nil, err
	}
	return channelInfo, nil
}

func addChannelIntoList(ctx context.Context, channelInfo *models.ChannelInfo) error {
	pubKey, _, err := getClientInfo(ctx, channelInfo.EncList[0].PkHash)
	if err != nil {
		return err
	}
	key, name, err := gClient.decoder.ChannelInfo(pubKey, channelInfo)
	if err != nil {
		return err
	}

	pubKeysMap := make(map[string]asymmetric.IPubKey, len(channelInfo.EncList)+1)
	pkHashes := make([]string, 0, len(channelInfo.EncList))
	for _, v := range channelInfo.EncList {
		pkHashes = append(pkHashes, v.PkHash)
		pubKey, _, err := getClientInfo(ctx, v.PkHash)
		if err != nil {
			return err
		}
		pubKeysMap[v.PkHash] = pubKey
	}

	gChannels.addChannel(&sChannel{
		isFavorite: gClient.isFavoriteChannel(channelInfo.ChanID),
		chanID:     channelInfo.ChanID,
		key:        key,
		aliasName:  name,
		pkHashes:   pkHashes,
		pubKeysMap: pubKeysMap,
	})
	return nil
}

func initLocalClient() (*models.ClientInfo, error) {
	pkHash := gClient.sk.GetPubKey().GetHasher().ToString()
	clientInfo, err := gClient.db.GetClient(pkHash)
	if err == nil {
		return clientInfo, nil
	}
	if !errors.Is(err, gp_database.ErrNotFound) {
		return nil, err
	}
	clientInfo = gClient.encoder.InitClient()
	if err := gClient.db.SetClient(clientInfo); err != nil {
		return nil, err
	}
	return clientInfo, nil
}

func initRemoteClient(ctx context.Context, clientInfo *models.ClientInfo) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	connsMapper := make(map[string]struct{})
	for {
		for _, c := range gClient.getConnections() {
			if _, ok := connsMapper[c.address]; ok {
				continue
			}
			connsMapper[c.address] = struct{}{}
			go func() {
				for {
					if err := newConn(c.address).InitClient(ctx, clientInfo); err != nil {
						fyne.Do(func() { printLog(logErro, err) })
						timeSleep(ctx, time.Minute)
						continue
					}
					break
				}
			}()
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func pushRemoteMessage(ctx context.Context, messageInfo *models.MessageInfo) error {
	var (
		lastErr    error
		hasSuccess bool
	)
	for _, c := range gClient.getConnections() {
		if err := newConn(c.address).PushMessage(ctx, messageInfo); err != nil {
			lastErr = err
			continue
		}
		hasSuccess = true
	}
	if hasSuccess {
		return nil
	}
	return lastErr
}

func getClientInfo(ctx context.Context, pkHash string) (asymmetric.IPubKey, *models.ClientInfo, error) {
	clientInfo, err := gClient.db.GetClient(pkHash)
	if err == nil {
		pubKey, err := gClient.decoder.ClientInfo(clientInfo)
		if err != nil {
			return nil, nil, err
		}
		return pubKey, clientInfo, nil
	}
	if !errors.Is(err, gp_database.ErrNotFound) {
		return nil, nil, err
	}
	var lastErr error
	for _, c := range gClient.getConnections() {
		clientInfo, err := newConn(c.address).LoadClient(ctx, pkHash)
		if err != nil {
			lastErr = err
			continue
		}
		pubKey, err := gClient.decoder.ClientInfo(clientInfo)
		if err != nil {
			lastErr = err
			continue
		}
		if err := gClient.db.SetClient(clientInfo); err != nil {
			return nil, nil, err
		}
		return pubKey, clientInfo, nil
	}
	return nil, nil, lastErr
}

func binarySearchCounter(ctx context.Context, channel *sChannel, appClient client.IClient, high int64) (uint64, error) {
	low := int64(0)
	result := int64(0)

	for low <= high {
		mid := low + (high-low)/2
		messageInfo, err := appClient.ListenMessage(ctx, channel.chanID, uint64(mid))
		if err != nil {
			return 0, err
		}
		msgHash := messageInfo.GetHash()
		_, err = gClient.db.GetMessage(msgHash)
		switch {
		case err == nil:
			// => next
			result = mid
			low = mid + 1
		case errors.Is(err, gp_database.ErrNotFound):
			// <= prev
			result = mid
			high = mid - 1
		default:
			return 0, err
		}
	}

	return uint64(result), nil
}

func newConn(addr string) client.IClient {
	return client.NewClient(addr, &http.Client{Timeout: 30 * time.Second})
}
