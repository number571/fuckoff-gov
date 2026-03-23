package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"image/color"
	"io"
	"net/url"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/number571/fuckoff-gov/internal/consts"
	"github.com/number571/go-peer/pkg/crypto/hashing"
)

func initWindowChatSearch(ctx context.Context, a fyne.App, w fyne.Window) *fyne.Container {
	header := widget.NewButtonWithIcon(
		"Back to channel chat",
		theme.ComputerIcon(),
		func() { setChatChanContent(ctx, w, currentChatChannel) },
	)

	scrollSearchContainer = newCustomScroller(container.NewVBox(), w)
	scrollSearchContainer.SetMinSize(fyne.NewSize(400, 300))

	inputConnectionEntry = widget.NewEntry()
	inputConnectionEntry.SetPlaceHolder("Type a string...")

	sendButton := widget.NewButtonWithIcon(
		"",
		theme.SearchIcon(),
		func() {
			// TODO:
			setChatSearchContent(w, currentChatChannel)
		},
	)

	inputConnectionEntry.OnSubmitted = func(s string) {
		sendButton.Tapped(nil)
	}

	inputEntrySendButton := container.New(
		layout.NewBorderLayout(nil, nil, nil, sendButton),
		inputConnectionEntry,
		sendButton,
	)

	content := container.New(
		layout.NewBorderLayout(header, inputEntrySendButton, nil, nil),
		header,
		scrollSearchContainer,
		inputEntrySendButton,
	)

	minSizeTarget := canvas.NewRectangle(color.Transparent)
	minSizeTarget.SetMinSize(fyne.NewSize(600, 400))

	contentContainerWrapper := container.New(
		layout.NewStackLayout(),
		minSizeTarget,
		content,
	)

	w.SetCloseIntercept(func() { a.Quit() })
	return contentContainerWrapper
}

func initWindowChatSettings(ctx context.Context, a fyne.App, w fyne.Window) *fyne.Container {
	header := widget.NewButtonWithIcon(
		"Back to channel chat",
		theme.ComputerIcon(),
		func() { setChatChanContent(ctx, w, currentChatChannel) },
	)

	participantsList := widget.NewList(
		func() int {
			return len(currentChatChannel.pkHashes)
		},
		func() fyne.CanvasObject {
			return container.NewVBox(widget.NewButton("", func() {}))
		},
		func(i widget.ListItemID, item fyne.CanvasObject) {
			pkHash := currentChatChannel.pkHashes[i]

			buttonName := item.(*fyne.Container).Objects[0].(*widget.Button)
			buttonName.SetText(cutHash384(pkHash))
			buttonName.OnTapped = func() {
				a.Clipboard().SetContent(pkHash)
				dialog.ShowInformation(
					"Copying a pk hash...",
					"The pk hash has been successfully copied to the clipboard",
					w,
				)
			}
		},
	)

	blockButton := widget.NewButtonWithIcon("Block chat", theme.CancelIcon(), func() {
		dialog.ShowConfirm(
			"Blocking chat...",
			"Are you sure you want to block this chat?",
			func(ok bool) {
				if !ok {
					return
				}
				setChatListContent(w)
			},
			w,
		)
	})
	blockButton.Importance = widget.DangerImportance

	favoriteButton := widget.NewButtonWithIcon("Favorite chat", theme.ConfirmIcon(), func() {
		dialog.ShowConfirm(
			"Chat to favorite...",
			"Are you sure you want set this chat to favorite list?",
			func(ok bool) {
				if !ok {
					return
				}
				setChatListContent(w)
			},
			w,
		)
	})
	favoriteButton.Importance = widget.SuccessImportance

	buttonsGrid := container.NewGridWithColumns(
		2,
		blockButton,
		favoriteButton,
	)

	content := container.New(
		layout.NewBorderLayout(header, buttonsGrid, nil, nil),
		header,
		buttonsGrid,
		participantsList,
	)

	minSizeTarget := canvas.NewRectangle(color.Transparent)
	minSizeTarget.SetMinSize(fyne.NewSize(600, 400))

	contentContainerWrapper := container.New(
		layout.NewStackLayout(),
		minSizeTarget,
		content,
	)

	w.SetCloseIntercept(func() { a.Quit() })
	return contentContainerWrapper
}

func initWindowAboutPage(_ context.Context, a fyne.App, w fyne.Window) *fyne.Container {
	header := widget.NewButtonWithIcon(
		"Back to main page",
		theme.ListIcon(),
		func() { setChatListContent(w) },
	)

	pkHash := gClient.sk.GetPubKey().GetHasher().ToString()
	pubKeyButton := widget.NewButtonWithIcon(
		cutHash384(pkHash),
		theme.ContentCopyIcon(),
		func() {
			a.Clipboard().SetContent(pkHash)
			dialog.ShowInformation(
				"Copying a pk hash...",
				"The pk hash has been successfully copied to the clipboard",
				w,
			)
		},
	)
	pubKeyButton.Importance = widget.MediumImportance

	coloredPubKeyButtonContainer := container.NewStack(
		canvas.NewRectangle(color.RGBA{R: 0, G: 0, B: 0, A: 100}),
		pubKeyButton,
	)

	versionGrid := container.NewGridWithColumns(
		2,
		widget.NewLabel("Version"),
		widget.NewLabel(consts.Version),
	)
	versionGrid.Objects[1].(*widget.Label).Importance = widget.WarningImportance

	coloredVersionGridContainer := container.NewStack(
		canvas.NewRectangle(color.RGBA{R: 0, G: 0, B: 0, A: 100}),
		versionGrid,
	)

	pkHashGrid := container.NewGridWithColumns(
		2,
		widget.NewLabel("PkHash"),
		coloredPubKeyButtonContainer,
	)

	inputNameEntry = widget.NewEntry()
	inputNameEntry.SetText(gClient.getNickName())
	inputNameEntry.OnChanged = func(s string) {
		if len(s) > consts.MaxNickNameSize {
			dialog.ShowError(fmt.Errorf("nickname size > max(%d)", consts.MaxNickNameSize), w)
			inputNameEntry.SetText(gClient.getNickName())
			return
		}
		if err := gClient.setNickName(s); err != nil {
			printLog(logErro, err)
		}
	}

	nicknameGrid := container.NewGridWithColumns(
		2,
		widget.NewLabel("Name"),
		inputNameEntry,
	)

	gridOfCommonInfo := container.NewGridWithRows(
		2,
		pkHashGrid,
		nicknameGrid,
	)

	coloredCommonInfoGridContainer := container.NewStack(
		canvas.NewRectangle(color.RGBA{R: 0, G: 0, B: 0, A: 100}),
		gridOfCommonInfo,
	)

	hyperlinkToAuthorWithLabel := container.NewGridWithColumns(
		2,
		widget.NewLabel("Author"),
		widget.NewHyperlink("github.com/number571", func() *url.URL {
			urlObj, _ := url.Parse("https://github.com/number571")
			return urlObj
		}()),
	)

	hyperlinkToProjectWithLabel := container.NewGridWithColumns(
		2,
		widget.NewLabel("Project"),
		widget.NewHyperlink("github.com/number571/fuckoff-gov", func() *url.URL {
			urlObj, _ := url.Parse("https://github.com/number571/fuckoff-gov")
			return urlObj
		}()),
	)

	gridOfHyperlinks := container.NewGridWithRows(
		2,
		hyperlinkToAuthorWithLabel,
		hyperlinkToProjectWithLabel,
	)

	coloredHyperlinkWithLabels := container.NewStack(
		canvas.NewRectangle(color.RGBA{R: 0, G: 0, B: 0, A: 100}),
		gridOfHyperlinks,
	)

	innerContent := container.NewVBox(
		coloredVersionGridContainer,
		coloredCommonInfoGridContainer,
		coloredHyperlinkWithLabels,
	)

	content := container.New(
		layout.NewBorderLayout(header, nil, nil, nil),
		header,
		innerContent,
	)

	minSizeTarget := canvas.NewRectangle(color.Transparent)
	minSizeTarget.SetMinSize(fyne.NewSize(600, 400))

	contentContainerWrapper := container.New(
		layout.NewStackLayout(),
		minSizeTarget,
		content,
	)

	w.SetCloseIntercept(func() { a.Quit() })
	return contentContainerWrapper
}

func initWindowAddChannels(ctx context.Context, a fyne.App, w fyne.Window) *fyne.Container {
	header := widget.NewButtonWithIcon(
		"Back to main page",
		theme.ListIcon(),
		func() { setChatListContent(w) },
	)

	inputPkHashEntry = widget.NewEntry()
	inputPkHashEntry.SetPlaceHolder("Type a pkhash...")

	sendButtonAddParticipant := widget.NewButtonWithIcon("", theme.ContentAddIcon(), func() {
		pkHash := inputPkHashEntry.Text
		if pkHash == "" || len(pkHash) != (hashing.CHasherSize<<1) {
			dialog.ShowError(errors.New("invalid pkhash"), w)
			return
		}
		gParticipants = append(gParticipants, pkHash)
		setEditChannelsContent(w)
	})

	inputPkHashEntry.OnSubmitted = func(s string) {
		sendButtonAddParticipant.Tapped(nil)
	}

	participantsList := widget.NewList(
		func() int { return len(gParticipants) },
		func() fyne.CanvasObject {
			templateDeleteButton := widget.NewButtonWithIcon("", theme.ContentClearIcon(), func() {})
			templateDeleteButton.Importance = widget.DangerImportance

			templateFriendButton := widget.NewButtonWithIcon("", theme.AccountIcon(), func() {})
			return container.New(
				layout.NewBorderLayout(nil, nil, templateDeleteButton, nil),
				templateDeleteButton,
				templateFriendButton,
			)
		},
		func(i widget.ListItemID, item fyne.CanvasObject) {
			participant := gParticipants[i]

			deleteButton := item.(*fyne.Container).Objects[0].(*widget.Button)
			deleteButton.OnTapped = func() {
				dialog.ShowConfirm(
					"Deleting participant...",
					"Are you sure you want to delete this participant?",
					func(ok bool) {
						if !ok {
							return
						}
						gParticipants = append(gParticipants[:i], gParticipants[i+1:]...)
						setEditChannelsContent(w)
					},
					w,
				)
			}

			friendButton := item.(*fyne.Container).Objects[1].(*widget.Button)
			friendButton.SetText(cutHash384(participant))
		},
	)

	inputChannelNameEntry = widget.NewEntry()
	inputChannelNameEntry.SetPlaceHolder("Type a channel name...")

	sendButtonCreateChannel := widget.NewButtonWithIcon("", theme.MailForwardIcon(), func() {
		channelName := inputChannelNameEntry.Text
		if channelName == "" {
			dialog.ShowError(errors.New("invalid channel name"), w)
			return
		}

		pkHashes := make([]string, len(gParticipants))
		copy(pkHashes, gParticipants)
		gParticipants = gParticipants[:0]

		channelInfo, err := initLocalChannel(ctx, channelName, pkHashes)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}

		if err := initRemoteChannel(ctx, channelInfo); err != nil {
			dialog.ShowError(err, w)
			return
		}

		dialog.ShowInformation("New channel", "Channel success created!", w)
		setEditChannelsContent(w)
	})
	sendButtonCreateChannel.Importance = widget.HighImportance

	inputEntryCreateChannel := container.New(
		layout.NewBorderLayout(nil, nil, sendButtonCreateChannel, nil),
		sendButtonCreateChannel,
		inputChannelNameEntry,
	)

	inputEntryContainer := container.New(
		layout.NewBorderLayout(inputEntryCreateChannel, nil, nil, sendButtonAddParticipant),
		inputEntryCreateChannel,
		inputPkHashEntry,
		sendButtonAddParticipant,
	)

	content := container.New(
		layout.NewBorderLayout(header, inputEntryContainer, nil, nil),
		header,
		participantsList,
		inputEntryContainer,
	)

	minSizeTarget := canvas.NewRectangle(color.Transparent)
	minSizeTarget.SetMinSize(fyne.NewSize(600, 400))

	contentContainerWrapper := container.New(
		layout.NewStackLayout(),
		minSizeTarget,
		content,
	)

	w.SetCloseIntercept(func() { a.Quit() })
	return contentContainerWrapper
}

func initWindowConnections(ctx context.Context, a fyne.App, w fyne.Window) *fyne.Container {
	header := widget.NewButtonWithIcon(
		"Back to main page",
		theme.ListIcon(),
		func() { setChatListContent(w) },
	)

	networksList := widget.NewList(
		func() int { return len(gClient.getConnections()) },
		func() fyne.CanvasObject {
			templateNetworkButton := widget.NewButton("", func() {})

			templateDeleteNetwork := widget.NewButtonWithIcon("", theme.ContentClearIcon(), func() {})
			templateDeleteNetwork.Importance = widget.DangerImportance

			return container.New(
				layout.NewBorderLayout(nil, nil, templateDeleteNetwork, nil),
				templateDeleteNetwork,
				templateNetworkButton,
			)
		},
		func(i widget.ListItemID, item fyne.CanvasObject) {
			connection := gClient.getConnections()[i]

			deleteButton := item.(*fyne.Container).Objects[0].(*widget.Button)
			deleteButton.OnTapped = func() {
				dialog.ShowConfirm(
					"Deleting connection...",
					"Are you sure you want to delete this connection?",
					func(ok bool) {
						if !ok {
							return
						}
						gClient.delConnection(connection.address)
						setConnectionsContent(ctx, w)
					},
					w,
				)
			}

			buttonName := item.(*fyne.Container).Objects[1].(*widget.Button)
			buttonName.SetText(connection.address)

			if connection.online {
				buttonName.Importance = widget.SuccessImportance
			}

			buttonName.OnTapped = func() {
				a.Clipboard().SetContent(connection.address)
				dialog.ShowInformation(
					"Copying a connection...",
					"The connection has been successfully copied to the clipboard",
					w,
				)
			}
		},
	)

	loggerLabel := widget.NewLabel("")
	loggerLabel.Selectable = true

	coloredLogLabelContainer := container.NewStack(
		canvas.NewRectangle(color.RGBA{R: 0, G: 0, B: 0, A: 100}),
		loggerLabel,
	)

	scrollLoggerLabel = container.NewScroll(coloredLogLabelContainer)
	scrollLoggerLabel.SetMinSize(fyne.NewSize(400, 100))

	gridBody := container.NewGridWithRows(
		2,
		networksList,
		scrollLoggerLabel,
	)

	inputConnectionEntry = widget.NewEntry()
	inputConnectionEntry.SetPlaceHolder("Type a connection...")

	sendButton := widget.NewButtonWithIcon(
		"",
		theme.MailForwardIcon(),
		func() {
			connection := inputConnectionEntry.Text
			inputConnectionEntry.SetText("")
			gClient.addConnection(connection)
			setConnectionsContent(ctx, w)
		},
	)

	inputConnectionEntry.OnSubmitted = func(s string) {
		sendButton.Tapped(nil)
	}

	inputEntrySendButton := container.New(
		layout.NewBorderLayout(nil, nil, nil, sendButton),
		inputConnectionEntry,
		sendButton,
	)

	content := container.New(
		layout.NewBorderLayout(header, inputEntrySendButton, nil, nil),
		header,
		gridBody,
		inputEntrySendButton,
	)

	minSizeTarget := canvas.NewRectangle(color.Transparent)
	minSizeTarget.SetMinSize(fyne.NewSize(600, 400))

	contentContainerWrapper := container.New(
		layout.NewStackLayout(),
		minSizeTarget,
		content,
	)

	w.SetCloseIntercept(func() { a.Quit() })
	return contentContainerWrapper
}

func initWindowChatChannel(ctx context.Context, a fyne.App, w fyne.Window) *fyne.Container {
	scrollChatContainer = newCustomScroller(container.NewVBox(), w)
	scrollChatContainer.SetMinSize(fyne.NewSize(400, 300))

	inputMessageEntry = widget.NewEntry()
	inputMessageEntry.SetPlaceHolder("Type a message...")

	fileButton := widget.NewButtonWithIcon("", theme.FileIcon(), func() {
		fileOpenDialog := dialog.NewFileOpen(
			func(reader fyne.URIReadCloser, err error) {
				if err != nil {
					dialog.ShowError(err, w)
					return
				}
				if reader == nil {
					return
				}
				defer reader.Close()

				filename := reader.URI().Name()
				if len(filename) > consts.MaxFileNameSize {
					dialog.ShowError(fmt.Errorf("file name > max(%d)", consts.MaxFileNameSize), w)
					return
				}

				content, err := io.ReadAll(reader)
				if err != nil {
					dialog.ShowError(err, w)
					return
				}

				comptessedContent, err := compressBytes(content)
				if err != nil {
					dialog.ShowError(err, w)
					return
				}

				if len(comptessedContent) > consts.MaxMessageSize {
					dialog.ShowError(fmt.Errorf("file size > max(%d)", consts.MaxMessageSize), w)
					return
				}

				pushMessage(ctx, currentChatChannel, filename, comptessedContent)
				inputMessageEntry.SetText("")
				w.Canvas().Focus(inputMessageEntry)
			},
			w,
		)
		fileOpenDialog.Show()
	})

	sendButton := widget.NewButtonWithIcon("", theme.MailSendIcon(), func() {
		content := inputMessageEntry.Text
		if content == "" {
			return
		}
		if len(content) > consts.MaxMessageSize {
			dialog.ShowError(fmt.Errorf("content size > max(%d)", consts.MaxMessageSize), w)
			return
		}
		pushMessage(ctx, currentChatChannel, "", []byte(content))
		inputMessageEntry.SetText("")
		w.Canvas().Focus(inputMessageEntry)
	})

	sendButtons := container.New(
		layout.NewBorderLayout(nil, nil, nil, sendButton),
		fileButton,
		sendButton,
	)

	inputBar := container.New(
		layout.NewBorderLayout(nil, nil, nil, sendButtons),
		inputMessageEntry,
		sendButtons,
	)

	inputMessageEntry.OnSubmitted = func(s string) {
		sendButton.Tapped(nil)
	}

	searchButton := widget.NewButtonWithIcon(
		"",
		theme.SearchIcon(),
		func() { setChatSearchContent(w, currentChatChannel) },
	)
	settingsButton := widget.NewButtonWithIcon(
		"",
		theme.MenuIcon(),
		func() { setChatSettingsContent(w, currentChatChannel) },
	)
	backToMainPageButton := widget.NewButtonWithIcon(
		"Back to main page",
		theme.ListIcon(),
		func() { setChatListContent(w) },
	)

	header := container.New(
		layout.NewBorderLayout(nil, nil, searchButton, settingsButton),
		backToMainPageButton,
		searchButton,
		settingsButton,
	)

	content := container.New(
		layout.NewBorderLayout(header, inputBar, nil, nil),
		header,
		inputBar,
		scrollChatContainer,
	)

	minSizeTarget := canvas.NewRectangle(color.Transparent)
	minSizeTarget.SetMinSize(fyne.NewSize(600, 400))

	contentContainerWrapper := container.New(
		layout.NewStackLayout(),
		minSizeTarget,
		content,
	)

	w.SetCloseIntercept(func() { a.Quit() })
	return contentContainerWrapper
}

func initWindowListChannels(ctx context.Context, a fyne.App, w fyne.Window) *fyne.Container {
	chatList := widget.NewList(
		func() int {
			return gChannels.getLength()
		},
		func() fyne.CanvasObject {
			return container.NewVBox(widget.NewButton("", func() {}))
		},
		func(i widget.ListItemID, item fyne.CanvasObject) {
			channel := gChannels.getChannels()[i]

			buttonName := item.(*fyne.Container).Objects[0].(*widget.Button)
			buttonName.SetText(fmt.Sprintf("%s [%s]", channel.aliasName, cutHash384(channel.chanID)))
			buttonName.OnTapped = func() { setChatChanContent(ctx, w, channel) }
		},
	)

	mainContentVBox := container.NewBorder(nil, nil, nil, nil, chatList)
	connectionsButton := widget.NewButtonWithIcon(
		"",
		theme.SettingsIcon(),
		func() { setConnectionsContent(ctx, w) },
	)
	aboutButton := widget.NewButtonWithIcon(
		"",
		theme.MenuIcon(),
		func() { setAboutContent(w) },
	)
	addChannelButton := widget.NewButtonWithIcon(
		"Add channel",
		theme.ComputerIcon(),
		func() { setEditChannelsContent(w) },
	)

	header := container.New(
		layout.NewBorderLayout(nil, nil, connectionsButton, aboutButton),
		addChannelButton,
		connectionsButton,
		aboutButton,
	)

	content := container.New(
		layout.NewBorderLayout(header, nil, nil, nil),
		header,
		mainContentVBox,
	)

	minSizeTarget := canvas.NewRectangle(color.Transparent)
	minSizeTarget.SetMinSize(fyne.NewSize(600, 400))

	contentContainerWrapper := container.New(
		layout.NewStackLayout(),
		minSizeTarget,
		content,
	)

	w.SetCloseIntercept(func() { a.Quit() })
	return contentContainerWrapper
}

type customScroller struct {
	container.Scroll
	mu       *sync.Mutex
	messages map[string]struct{}
	switched bool
	w        fyne.Window
}

func newCustomScroller(content fyne.CanvasObject, w fyne.Window) *customScroller {
	s := &customScroller{}
	s.Content = content
	s.mu = &sync.Mutex{}
	s.messages = make(map[string]struct{}, 4096)
	s.switched = true
	s.w = w
	s.ExtendBaseWidget(s)
	return s
}

// Scrolled is called whenever the scroll position changes
func (s *customScroller) Scrolled(ev *fyne.ScrollEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Scroll.Scrolled(ev)

	if s.Offset.Y <= 0 && s.switched {
		if startIndexReader == 0 {
			s.switched = false
			return
		}

		readUntil := int64(-1)
		if startIndexReader > consts.CountMessagesPerPage {
			readUntil = int64(startIndexReader - consts.CountMessagesPerPage)
		}

		index := int64(startIndexReader)
		startIndexReader = uint64(readUntil)

		for index > int64(readUntil) {
			if index < 0 {
				break
			}
			msgHash, err := gClient.db.GetChannelMessageHashByIndex(currentChatChannel.chanID, uint64(index))
			if err != nil {
				fyne.Do(func() { dialog.ShowError(err, s.w) })
				return
			}
			if _, ok := s.messages[msgHash]; ok {
				index--
				continue
			}
			s.messages[msgHash] = struct{}{}
			messageInfo, err := gClient.db.GetMessage(msgHash)
			if err != nil {
				fyne.Do(func() { dialog.ShowError(err, s.w) })
				return
			}
			pubKey, ok := currentChatChannel.pubKeysMap[messageInfo.PkHash]
			if !ok {
				fyne.Do(func() { dialog.ShowError(err, s.w) })
				return
			}
			msgBody, err := gClient.decoder.MessageInfo(pubKey, currentChatChannel.key, messageInfo)
			if err != nil {
				fyne.Do(func() { dialog.ShowError(err, s.w) })
				return
			}
			index--
			fyne.Do(func() { addMessageToChat(s.w, pubKey, msgBody, true) })
		}

		s.switched = false
	}
	if s.Offset.Y > 0 {
		s.messages = make(map[string]struct{}, 4096)
		s.switched = true
	}

}

func cutHash384(pkHash string) string {
	return fmt.Sprintf("%s...%s", pkHash[:12], pkHash[len(pkHash)-12:])
}

func compressBytes(data []byte) ([]byte, error) {
	var b bytes.Buffer
	zw := gzip.NewWriter(&b)
	if _, err := zw.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write data to gzip writer: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}
	return b.Bytes(), nil
}

func decompressBytes(data []byte) ([]byte, error) {
	buf := bytes.NewReader(data)
	zr, err := gzip.NewReader(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer zr.Close()
	decompressedData, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("failed to read decompressed data: %w", err)
	}
	return decompressedData, nil
}
