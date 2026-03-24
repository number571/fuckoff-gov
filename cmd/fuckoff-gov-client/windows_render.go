package main

import (
	"bytes"
	"context"
	"crypto/x509"
	"fmt"
	"image/color"
	"image/jpeg"
	"image/png"
	"path/filepath"
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
	favoriteChanButton    *widget.Button
	inputNameEntry        *widget.Entry
	inputPkHashEntry      *widget.Entry
	inputChannelNameEntry *widget.Entry
	inputChatSearchEntry  *widget.Entry
	inputMessageEntry     *widget.Entry
	scrollChatContainer   *customScroller
	scrollSearchContainer *customScroller
	scrollLoggerLabel     *container.Scroll
)

var (
	chatListenerActive     = false
	startChatIndexReader   = uint64(0)
	startSearchIndexReader = uint64(0)
	closeListenChat        = make(chan struct{})
	currentChatChannel     *sChannel
)

type sConnection struct {
	online bool
	id     string
	cert   *x509.Certificate
}

var (
	gParticipants = []string{}
)

func setChatSearchContent(w fyne.Window, channel *sChannel) {
	clearAfterSwitch()
	currentChatChannel = channel

	w.SetContent(chatSearchContainer)
	w.Canvas().Focus(inputChatSearchEntry)
}

func setChatSettingsContent(w fyne.Window, channel *sChannel) {
	clearAfterSwitch()
	currentChatChannel = channel

	if currentChatChannel.isFavorite {
		favoriteChanButton.Icon = theme.CancelIcon()
		favoriteChanButton.Importance = widget.MediumImportance
	} else {
		favoriteChanButton.Icon = theme.ConfirmIcon()
		favoriteChanButton.Importance = widget.HighImportance
	}

	favoriteChanButton.OnTapped = func() {
		if currentChatChannel.isFavorite {
			dialog.ShowConfirm(
				"Delete from favorite...",
				"Are you sure you want to delete this chat from favorite list?",
				func(ok bool) {
					if !ok {
						return
					}

					if err := gClient.unsetFavoriteChannel(currentChatChannel.chanID); err != nil {
						dialog.ShowError(err, w)
						return
					}

					currentChatChannel.isFavorite = false
					favoriteChanButton.Importance = widget.HighImportance
					gChannels.sortByFavorites()

					setChatSettingsContent(w, currentChatChannel)
				},
				w,
			)
		} else {
			dialog.ShowConfirm(
				"Add to favorite...",
				"Are you sure you want add this chat to favorite list?",
				func(ok bool) {
					if !ok {
						return
					}

					if err := gClient.setFavoriteChannel(currentChatChannel.chanID); err != nil {
						dialog.ShowError(err, w)
						return
					}

					currentChatChannel.isFavorite = true
					favoriteChanButton.Importance = widget.MediumImportance
					gChannels.sortByFavorites()

					setChatSettingsContent(w, currentChatChannel)
				},
				w,
			)
		}
	}

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

	go runMessagesListener(ctx, w, channel)

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
	startChatIndexReader = 0
	startSearchIndexReader = 0
	inputChatSearchEntry.SetText("")
	inputChannelNameEntry.SetText("")
	inputPkHashEntry.SetText("")
	inputMessageEntry.SetText("")
	scrollChatContainer.messages = make(map[string]struct{}, 4096)
	scrollChatContainer.Content.(*fyne.Container).RemoveAll()
	scrollSearchContainer.messages = make(map[string]struct{}, 4096)
	scrollSearchContainer.Content.(*fyne.Container).RemoveAll()
	if chatListenerActive {
		closeListenChat <- struct{}{}
	}
}

func pingConnections(ctx context.Context) {
	for _, c := range gClient.getConnections() {
		c.online = (newConn(c.cert).Ping(ctx) == nil)
	}
}

func pushMessage(ctx context.Context, channel *sChannel, filename string, payload []byte) error {
	messageInfo, err := gClient.encoder.PushMessage(
		channel.chanID,
		channel.key,
		&models.MessageBody{
			Filename:  filename,
			Sender:    gClient.getNickName(),
			Payload:   payload,
			Timestamp: time.Now(),
		},
	)
	if err != nil {
		return err
	}
	if err := pushRemoteMessage(ctx, messageInfo); err != nil {
		return err
	}
	return nil
}

func addMessageToChat(w fyne.Window, scrollContainer *customScroller, pkSender asymmetric.IPubKey, msgBody *models.MessageBody, toTop bool) {
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
			msgLabel := widget.NewLabel(fmt.Sprintf("%s [%s]", cutHash384(pkSenderHash), msgBody.Timestamp.Format(time.DateTime)))
			msgLabel.Wrapping = fyne.TextWrapWord
			msgLabel.Selectable = true
			msgLabel.Importance = widget.LowImportance
			return msgLabel
		}(),
	)

	bgColor := color.NRGBA{R: 0, G: 0, B: 0, A: 128}
	backgroundRect := canvas.NewRectangle(bgColor)
	coloredContainer := container.NewStack(backgroundRect, c)

	contentContainer := scrollContainer.Content.(*fyne.Container)
	if toTop {
		contentContainer.Objects = append([]fyne.CanvasObject{coloredContainer}, contentContainer.Objects...)
		scrollContainer.ScrollToTop()
	} else {
		contentContainer.Objects = append(contentContainer.Objects, coloredContainer)
		if isAtBottom(scrollContainer) {
			scrollContainer.ScrollToBottom()
		}
	}
}

func isAtBottom(scroll *customScroller) bool {
	diff := float32(400)
	maxY := scroll.Content.MinSize().Height - scroll.Size().Height - diff
	if maxY <= 0 {
		return true
	}
	return scroll.Offset.Y >= maxY
}

func getMessageAsText(_ fyne.Window, msgBody *models.MessageBody) *widget.Label {
	msgLabel := widget.NewLabel(string(msgBody.Payload))
	msgLabel.Wrapping = fyne.TextWrapWord
	msgLabel.Selectable = true
	return msgLabel
}

func getMessageAsFile(w fyne.Window, msgBody *models.MessageBody) *fyne.Container {
	var err error

	filename := msgBody.Filename
	payload := msgBody.Payload

	if !fileIsImage(filename) {
		payload, err = decompressBytes(msgBody.Payload)
	} else {
		imgReader := bytes.NewReader(msgBody.Payload)
		if filepath.Ext(filename) == "png" {
			_, err = png.Decode(imgReader)
		} else {
			_, err = jpeg.Decode(imgReader)
		}
	}

	if err != nil {
		printLog(logErro, err)
		return container.New(
			layout.NewVBoxLayout(),
			getFileAsBinary(fmt.Sprintf("%s (invalid bytes)", filename)),
		)
	}

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
					if _, err := writer.Write(payload); err != nil {
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
		data = getFileAsImage(filename, payload)
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
