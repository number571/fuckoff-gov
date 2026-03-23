package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/color"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/number571/fuckoff-gov/internal/models"
	"github.com/number571/go-peer/pkg/crypto/asymmetric"
)

var (
	inputNameEntry        *widget.Entry
	inputPkHashEntry      *widget.Entry
	inputChannelNameEntry *widget.Entry
	inputConnectionEntry  *widget.Entry
	inputMessageEntry     *widget.Entry
	scrollChatContainer   *customScroller
	scrollSearchContainer *customScroller
	scrollLoggerLabel     *container.Scroll
)

var (
	chatListenerActive = false
	closeListenChat    = make(chan struct{})
	currentChatChannel *sChannel
)

const (
	countPage = 128
)

type sConnection struct {
	online  bool
	address string
}

var (
	gParticipants = []string{}
	gChannels     = newChannelsList()
)

func setChatSearchContent(w fyne.Window, channel *sChannel) {
	clearAfterSwitch()
	currentChatChannel = channel

	w.SetContent(chatSearchContainer)
}

func setChatSettingsContent(w fyne.Window, channel *sChannel) {
	clearAfterSwitch()
	currentChatChannel = channel

	w.SetContent(chatSettingsContainer)
}

func setAboutContent(w fyne.Window) {
	clearAfterSwitch()

	inputNameEntry.SetText(gClient.getNickName())
	w.SetContent(aboutPageContainer)
}

func setConnectionsContent(ctx context.Context, w fyne.Window) {
	clearAfterSwitch()
	pingConnections(ctx)

	w.SetContent(connectionsContainer)
	w.Canvas().Focus(inputConnectionEntry)

	go func() {
		fyne.Do(func() {
			timeSleep(ctx, 100*time.Millisecond)
			scrollLoggerLabel.ScrollToBottom()
		})
	}()
}

func setEditChannelsContent(w fyne.Window) {
	clearAfterSwitch()

	w.SetContent(addChannelsContainer)
	w.Canvas().Focus(inputPkHashEntry)
}

func setChatListContent(w fyne.Window) {
	clearAfterSwitch()

	w.SetContent(listChannelsContainer)
}

func setChatChanContent(ctx context.Context, w fyne.Window, channel *sChannel) {
	clearAfterSwitch()
	currentChatChannel = channel

	listenMessages(w, channel)

	w.SetContent(chatChannelContainer)
	w.Canvas().Focus(inputMessageEntry)

	go func() {
		fyne.Do(func() {
			timeSleep(ctx, 100*time.Millisecond)
			scrollChatContainer.ScrollToBottom()
		})
	}()
}

func clearAfterSwitch() {
	inputChannelNameEntry.SetText("")
	inputConnectionEntry.SetText("")
	inputPkHashEntry.SetText("")
	inputMessageEntry.SetText("")
	scrollChatContainer.Content.(*fyne.Container).RemoveAll()
	if chatListenerActive {
		closeListenChat <- struct{}{}
	}
}

func pingConnections(ctx context.Context) {
	for _, c := range gClient.getConnections() {
		c.online = (newConn(c.address).Ping(ctx) == nil)
	}
}

func pushMessage(ctx context.Context, w fyne.Window, channel *sChannel, filename string, payload []byte) {
	msgBody := &models.MessageBody{
		Filename:  filename,
		Sender:    gClient.getNickName(),
		Payload:   payload,
		Timestamp: time.Now(),
	}
	messageInfo := gClient.encoder.PushMessage(
		channel.chanID,
		channel.key,
		msgBody,
	)
	for _, c := range gClient.getConnections() {
		if err := newConn(c.address).PushMessage(ctx, messageInfo); err != nil {
			printLog(logErro, err)
			continue
		}
	}
}

func listenMessages(w fyne.Window, channel *sChannel) {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		chatListenerActive = true
		<-closeListenChat
		chatListenerActive = false
		cancel()
	}()

	for _, c := range gClient.getConnections() {
		appClient := newConn(c.address)

		counter, err := appClient.CountMessages(ctx, channel.chanID)
		if err != nil {
			printLog(logErro, err)
			continue
		}

		if counter > countPage {
			counter -= countPage
		} else {
			counter = 0
		}

		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				msgInfo, err := appClient.ListenMessage(ctx, channel.chanID, counter)
				if err != nil {
					fyne.Do(func() { printLog(logErro, err) })
					timeSleep(ctx, time.Second)
					continue
				}
				pubKey, ok := channel.pubKeysMap[msgInfo.PkHash]
				if !ok {
					fyne.Do(func() { printLog(logErro, errors.New("pubkey not found")) })
					timeSleep(ctx, time.Second)
					continue
				}
				msgBody, err := gClient.decoder.MessageInfo(pubKey, channel.key, msgInfo)
				if err != nil {
					fyne.Do(func() { printLog(logErro, err) })
					timeSleep(ctx, time.Second)
					continue
				}
				counter++
				fyne.Do(func() { addMessageToChat(w, pubKey, msgBody, false) })
			}
		}()
	}
}

func addMessageToChat(w fyne.Window, pkSender asymmetric.IPubKey, msgBody *models.MessageBody, toTop bool) {
	pkSenderHash := pkSender.GetHasher().ToString()

	var data fyne.CanvasObject
	if msgBody.Filename != "" {
		data = getMessageAsFile(w, msgBody)
	} else {
		data = getMessageAsText(w, msgBody)
	}

	c := container.New(
		layout.NewCustomPaddedVBoxLayout(0.1),
		func() *widget.Label {
			isIncoming := (pkSenderHash != gClient.sk.GetPubKey().GetHasher().ToString())
			msgLabel := widget.NewLabel(msgBody.Sender)
			msgLabel.Wrapping = fyne.TextWrapWord
			msgLabel.Selectable = true
			msgLabel.Importance = widget.HighImportance
			if isIncoming {
				msgLabel.Importance = widget.DangerImportance
			}
			return msgLabel
		}(),
		data,
		func() *widget.Label {
			msgLabel := widget.NewLabel(fmt.Sprintf("%s [%s]", cutPkHash(pkSenderHash), msgBody.Timestamp.Format(time.DateTime)))
			msgLabel.Wrapping = fyne.TextWrapWord
			msgLabel.Selectable = true
			msgLabel.Importance = widget.LowImportance
			return msgLabel
		}(),
	)

	bgColor := color.NRGBA{R: 0, G: 0, B: 0, A: 128}
	backgroundRect := canvas.NewRectangle(bgColor)
	coloredContainer := container.NewStack(backgroundRect, c)

	contentContainer := scrollChatContainer.Content.(*fyne.Container)
	if toTop {
		contentContainer.Objects = append([]fyne.CanvasObject{coloredContainer}, contentContainer.Objects...)
	} else {
		contentContainer.Objects = append(contentContainer.Objects, coloredContainer)
	}

	scrollChatContainer.switched = true
	scrollChatContainer.ScrollToBottom()
}

func getMessageAsText(_ fyne.Window, msgBody *models.MessageBody) *widget.Label {
	msgLabel := widget.NewLabel(string(msgBody.Payload))
	msgLabel.Wrapping = fyne.TextWrapWord
	msgLabel.Selectable = true
	return msgLabel
}

func getMessageAsFile(w fyne.Window, msgBody *models.MessageBody) *fyne.Container {
	filename := msgBody.Filename

	downloadButton := widget.NewButtonWithIcon("LOAD", theme.DownloadIcon(), func() {
		fileDialog := dialog.NewFileSave(
			func(writer fyne.URIWriteCloser, err error) {
				if err != nil {
					dialog.ShowError(err, w)
					return
				}
				if writer == nil {
					return
				}
				go func() {
					defer writer.Close()
					if _, err := writer.Write(msgBody.Payload); err != nil {
						dialog.ShowError(err, w)
						return
					}
					dialog.ShowInformation("Download state", "File was successfully downloaded", w)
				}()
			},
			w,
		)
		fileDialog.SetFileName(filename)
		fileDialog.Show()
	})
	downloadButton.Importance = widget.LowImportance

	var data fyne.CanvasObject
	if fileIsImage(filename) {
		data = getFileAsImage(filename, msgBody.Payload)
	} else {
		data = getFileAsBinary(filename)
	}

	return container.New(
		layout.NewBorderLayout(nil, nil, nil, downloadButton),
		data,
		downloadButton,
	)
}

func getFileAsImage(filename string, body []byte) fyne.CanvasObject {
	image := canvas.NewImageFromReader(bytes.NewReader(body), filename)
	if image == nil {
		return getFileAsBinary(filename)
	}
	image.FillMode = canvas.ImageFillContain
	bg := canvas.NewRectangle(color.Black)
	bg.SetMinSize(fyne.NewSize(400, 400))
	return container.NewStack(bg, image)
}

func getFileAsBinary(filename string) fyne.CanvasObject {
	msgLabel := widget.NewLabel(filename)

	msgLabel.Importance = widget.WarningImportance
	msgLabel.Wrapping = fyne.TextWrapWord
	msgLabel.Selectable = true

	return msgLabel
}

func timeSleep(ctx context.Context, n time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(n):
	}
}

func fileIsImage(filename string) bool {
	imageExt := []string{".png", ".jpg", ".jpeg"}
	for _, v := range imageExt {
		if strings.HasSuffix(filename, v) {
			return true
		}
	}
	return false
}
