package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net/url"
	"path/filepath"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/number571/fuckoff-gov/internal/consts"
	"github.com/number571/fuckoff-gov/internal/models"
	"github.com/number571/go-peer/pkg/crypto/hashing"
	"golang.org/x/image/webp"
)

func initWindowChatSearch(ctx context.Context, a fyne.App, w fyne.Window) *fyne.Container {
	header := widget.NewButtonWithIcon(
		"Back to channel chat",
		theme.ComputerIcon(),
		func() { setChatChanContent(ctx, w, currentChatChannel) },
	)

	scrollSearchContainer = newCustomScroller(container.NewVBox(), &startSearchIndexReader, w)
	scrollSearchContainer.SetMinSize(fyne.NewSize(400, 300))

	inputChatSearchEntry = widget.NewEntry()
	inputChatSearchEntry.SetPlaceHolder("Type a string...")

	inputChatSearchEntry.OnSubmitted = func(s string) {
		if inputChatSearchEntry.Text == "" {
			return
		}

		text := []byte(inputChatSearchEntry.Text)
		filter := func(mb *models.MessageBody) bool {
			if mb.Filename != "" {
				return false
			}
			return bytes.Contains(mb.Payload, text)
		}

		setChatSearchContent(w, currentChatChannel)

		counter, err := gClient.db.GetCountChannelMessages(currentChatChannel.chanID)
		if err != nil {
			fyne.Do(func() { dialog.ShowError(err, w) })
			return
		}

		readCount := 0
		index := int64(counter) - 1
		for {
			if index < 0 || readCount == consts.CountMessagesPerPage {
				break
			}
			msgHash, err := gClient.db.GetChannelMessageHashByIndex(currentChatChannel.chanID, uint64(index))
			if err != nil {
				fyne.Do(func() { dialog.ShowError(err, w) })
				return
			}
			messageInfo, err := gClient.db.GetMessage(msgHash)
			if err != nil {
				fyne.Do(func() { dialog.ShowError(err, w) })
				return
			}
			pubKey, ok := currentChatChannel.pubKeysMap[messageInfo.PkHash]
			if !ok {
				fyne.Do(func() { dialog.ShowError(err, w) })
				return
			}
			msgBody, err := gClient.decoder.MessageInfo(messageInfo, pubKey, currentChatChannel.pkHashes, currentChatChannel.key)
			if err != nil {
				fyne.Do(func() { dialog.ShowError(err, w) })
				return
			}
			index--
			if !filter(msgBody) {
				continue
			}
			readCount++
			fyne.Do(func() { addMessageToChat(w, scrollSearchContainer, pubKey, msgBody, true) })
		}

		if index <= 0 {
			startSearchIndexReader = 0
		} else {
			startSearchIndexReader = uint64(index)
		}

		scrollSearchContainer.filter = filter
		scrollSearchContainer.ScrollToBottom()
	}

	content := container.New(
		layout.NewBorderLayout(header, inputChatSearchEntry, nil, nil),
		header,
		scrollSearchContainer,
		inputChatSearchEntry,
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
			templateDeleteButton := widget.NewButton("", func() {})
			templateDeleteButton.Importance = widget.DangerImportance

			templateFriendButton := widget.NewButton("", func() {})
			return container.New(
				layout.NewBorderLayout(nil, nil, templateDeleteButton, nil),
				templateDeleteButton,
				templateFriendButton,
			)
		},
		func(i widget.ListItemID, item fyne.CanvasObject) {
			pkHash := currentChatChannel.pkHashes[i]

			deleteButton := item.(*fyne.Container).Objects[0].(*widget.Button)

			if gClient.isBlockedParticipant(pkHash) {
				deleteButton.Icon = theme.ContentClearIcon()
				deleteButton.Importance = widget.MediumImportance
			} else {
				deleteButton.Icon = theme.AccountIcon()
				deleteButton.Importance = widget.DangerImportance
			}

			if pkHash == gClient.sk.GetPubKey().GetHasher().ToString() {
				deleteButton.Disable()
			}
			deleteButton.Refresh()

			if gClient.isBlockedParticipant(pkHash) {
				deleteButton.OnTapped = func() {
					dialog.ShowConfirm(
						"Unblocking participant...",
						"Are you sure you want to unblock this participant?",
						func(ok bool) {
							if !ok {
								return
							}
							if err := gClient.unsetBlockedParticipant(pkHash); err != nil {
								dialog.ShowError(err, w)
								return
							}
							deleteButton.Icon = theme.AccountIcon()
							deleteButton.Importance = widget.DangerImportance
							deleteButton.Refresh()
							setChatSettingsContent(w, currentChatChannel)
						},
						w,
					)
				}
			} else {
				deleteButton.OnTapped = func() {
					dialog.ShowConfirm(
						"Blocking participant...",
						"Are you sure you want to block this participant?",
						func(ok bool) {
							if !ok {
								return
							}
							if err := gClient.setBlockedParticipant(pkHash); err != nil {
								dialog.ShowError(err, w)
								return
							}
							deleteButton.Icon = theme.ContentClearIcon()
							deleteButton.Importance = widget.MediumImportance
							deleteButton.Refresh()
							setChatSettingsContent(w, currentChatChannel)
						},
						w,
					)
				}
			}

			friendButton := item.(*fyne.Container).Objects[1].(*widget.Button)
			friendButton.SetText(cutHash384(pkHash))
			friendButton.OnTapped = func() {
				a.Clipboard().SetContent(pkHash)
				dialog.ShowInformation(
					"Copying a pk hash...",
					"The pk hash has been successfully copied to the clipboard",
					w,
				)
			}
		},
	)

	deleteButton := widget.NewButtonWithIcon("Delete chat", theme.DeleteIcon(), func() {
		dialog.ShowConfirm(
			"Deleting chat...",
			"Are you sure you want to delete this chat?",
			func(ok bool) {
				chanID := currentChatChannel.chanID
				if !ok {
					return
				}
				if err := gClient.setDeletedChannel(chanID); err != nil {
					dialog.ShowError(err, w)
					return
				}
				if err := gClient.db.DelChannel(chanID); err != nil {
					dialog.ShowError(err, w)
					return
				}
				gClient.channels.delChannel(chanID)
				setChatListContent(w)
			},
			w,
		)
	})
	deleteButton.Importance = widget.DangerImportance

	favoriteChanButton = widget.NewButton("Favorite chat", func() {})

	buttonsGrid := container.NewGridWithColumns(
		2,
		deleteButton,
		favoriteChanButton,
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

	versionGrid := container.New(
		&ratioLayout{ratios: []float32{0.2, 0.8}},
		widget.NewLabel("Version"),
		widget.NewLabel(consts.Version),
	)
	versionGrid.Objects[1].(*widget.Label).Importance = widget.WarningImportance

	coloredVersionGridContainer := container.NewStack(
		canvas.NewRectangle(color.RGBA{R: 0, G: 0, B: 0, A: 100}),
		versionGrid,
	)

	pkHashGrid := container.New(
		&ratioLayout{ratios: []float32{0.2, 0.8}},
		widget.NewLabel("PkHash"),
		coloredPubKeyButtonContainer,
	)

	inputNameEntry = widget.NewEntry()
	inputNameEntry.SetText(gClient.getNickName())
	inputNameEntry.OnSubmitted = func(s string) {
		if len(s) > consts.MaxNickNameSize {
			dialog.ShowError(fmt.Errorf("nickname size > max(%d)", consts.MaxNickNameSize), w)
			inputNameEntry.SetText(gClient.getNickName())
			return
		}
		if err := gClient.setNickName(s); err != nil {
			printLog(logErro, err)
		}
	}

	nicknameGrid := container.New(
		&ratioLayout{ratios: []float32{0.2, 0.8}},
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

	hyperlinkToAuthorWithLabel := container.New(
		&ratioLayout{ratios: []float32{0.2, 0.8}},
		widget.NewLabel("Author"),
		widget.NewHyperlink("github.com/number571", func() *url.URL {
			urlObj, _ := url.Parse("https://github.com/number571")
			return urlObj
		}()),
	)

	hyperlinkToProjectWithLabel := container.New(
		&ratioLayout{ratios: []float32{0.2, 0.8}},
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

func initWindowAddChannel(ctx context.Context, a fyne.App, w fyne.Window) *fyne.Container {
	header := widget.NewButtonWithIcon(
		"Back to main page",
		theme.ListIcon(),
		func() { setChatListContent(w) },
	)

	inputPkHashEntry = widget.NewEntry()
	inputPkHashEntry.SetPlaceHolder("Type a pkhash...")

	inputPkHashEntry.OnSubmitted = func(s string) {
		defer w.Canvas().Unfocus()
		defer inputPkHashEntry.SetText("")

		pkHash := inputPkHashEntry.Text
		if pkHash == "" || len(pkHash) != (hashing.CHasherSize<<1) {
			dialog.ShowError(errors.New("invalid pkhash"), w)
			return
		}

		gParticipants = append(gParticipants, pkHash)
		setAddChannelContent(w)
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
						setAddChannelContent(w)
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

	inputChannelNameEntry.OnSubmitted = func(s string) {
		dialog.ShowConfirm(
			"Create channel",
			"It may take several minutes to create a channel...",
			func(ok bool) {
				defer w.Canvas().Unfocus()

				if !ok {
					return
				}

				defer inputChannelNameEntry.SetText("")
				channelName := inputChannelNameEntry.Text
				if channelName == "" {
					dialog.ShowError(errors.New("invalid channel name"), w)
					return
				}

				pkHashes := make([]string, len(gParticipants))
				copy(pkHashes, gParticipants)
				gParticipants = gParticipants[:0]

				go func() {
					channelInfo, err := initLocalChannel(ctx, channelName, pkHashes)
					if err != nil {
						fyne.Do(func() { dialog.ShowError(err, w) })
						return
					}

					if err := initRemoteChannel(ctx, channelInfo); err != nil {
						fyne.Do(func() { dialog.ShowError(err, w) })
						return
					}

					if err := gClient.db.SetChannel(channelInfo); err != nil {
						fyne.Do(func() { dialog.ShowError(err, w) })
						return
					}

					if err := addChannelIntoList(ctx, channelInfo); err != nil {
						fyne.Do(func() { dialog.ShowError(err, w) })
						return
					}

					fyne.Do(func() { channelsList.Refresh() })
					fyne.Do(func() { dialog.ShowInformation("New channel", "Channel success created!", w) })
				}()

				setAddChannelContent(w)
			},
			w,
		)
	}

	inputEntryCreateChannel := container.NewGridWithColumns(
		2,
		inputPkHashEntry,
		inputChannelNameEntry,
	)

	content := container.New(
		layout.NewBorderLayout(header, inputEntryCreateChannel, nil, nil),
		header,
		participantsList,
		inputEntryCreateChannel,
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
			c := gClient.getConnections()[i]

			deleteButton := item.(*fyne.Container).Objects[0].(*widget.Button)
			deleteButton.OnTapped = func() {
				dialog.ShowConfirm(
					"Deleting connection...",
					"Are you sure you want to delete this connection?",
					func(ok bool) {
						if !ok {
							return
						}
						gClient.delConnection(c.id)
						setConnectionsContent(ctx, w)
					},
					w,
				)
			}

			buttonName := item.(*fyne.Container).Objects[1].(*widget.Button)
			buttonName.SetText(cutHash384(c.id))

			if c.online {
				buttonName.Importance = widget.SuccessImportance
				buttonName.Refresh()
			}

			buttonName.OnTapped = func() {
				a.Clipboard().SetContent(string(certToBytes(c.cert)))
				dialog.ShowInformation(
					"Copying a connection...",
					"The connection has been successfully copied to the clipboard",
					w,
				)
			}
		},
	)

	loggerLabel := widget.NewLabel("")
	// loggerLabel.Selectable = true

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

	connectionLoadButton := widget.NewButtonWithIcon("Load certificate", theme.FileIcon(), func() {
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

				certBytes, err := io.ReadAll(reader)
				if err != nil {
					dialog.ShowError(err, w)
					return
				}

				cert, err := bytesToCert(certBytes)
				if err != nil {
					dialog.ShowError(err, w)
					return
				}

				clientInfo, err := initLocalClient()
				if err != nil {
					dialog.ShowError(err, w)
					return
				}

				certID, err := gClient.addConnection(cert)
				if err != nil {
					dialog.ShowError(err, w)
					return
				}

				conn, ok := gClient.getConnectionByID(certID)
				if !ok {
					dialog.ShowError(errors.New("connection not found"), w)
					return
				}

				if err := conn.client.InitClient(ctx, clientInfo); err != nil {
					dialog.ShowError(err, w)
					return
				}
				if err := conn.client.Auth(ctx); err != nil {
					dialog.ShowError(err, w)
					return
				}

				setConnectionsContent(ctx, w)
			},
			w,
		)
		fileOpenDialog.Show()
	})

	content := container.New(
		layout.NewBorderLayout(header, connectionLoadButton, nil, nil),
		header,
		gridBody,
		connectionLoadButton,
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
	scrollChatContainer = newCustomScroller(container.NewVBox(), &startChatIndexReader, w)
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

				var compressedContent []byte

				if !fileIsImage(filename) {
					compressedContent, err = compressBytes(content)
					if err != nil {
						dialog.ShowError(err, w)
						return
					}
				} else {
					var (
						img image.Image
						buf bytes.Buffer
					)
					imgReader := bytes.NewReader(content)
					addExtension := ".jpg"
					switch filepath.Ext(filename) {
					case ".png":
						img, err = png.Decode(imgReader)
						if err != nil {
							dialog.ShowError(err, w)
							return
						}
					case ".webp":
						img, err = webp.Decode(imgReader)
						if err != nil {
							dialog.ShowError(err, w)
							return
						}
					default:
						addExtension = ""
						img, err = jpeg.Decode(imgReader)
						if err != nil {
							dialog.ShowError(err, w)
							return
						}
					}
					filename += addExtension
					if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 25}); err != nil {
						dialog.ShowError(err, w)
						return
					}
					compressedContent = buf.Bytes()
				}

				go func() {
					if err := pushMessage(ctx, currentChatChannel, filename, compressedContent); err != nil {
						fyne.Do(func() { dialog.ShowError(err, w) })
						return
					}
				}()

				// inputMessageEntry.SetText("")
				// w.Canvas().Focus(inputMessageEntry)
			},
			w,
		)
		fileOpenDialog.Show()
	})

	inputBar := container.New(
		layout.NewBorderLayout(nil, nil, nil, fileButton),
		inputMessageEntry,
		fileButton,
	)

	inputMessageEntry.OnSubmitted = func(s string) {
		content := inputMessageEntry.Text
		if content == "" {
			return
		}
		go func() {
			if err := pushMessage(ctx, currentChatChannel, "", []byte(content)); err != nil {
				fyne.Do(func() { dialog.ShowError(err, w) })
				return
			}
		}()
		inputMessageEntry.SetText("")
		// w.Canvas().Focus(inputMessageEntry)
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
	channelsList = widget.NewList(
		func() int {
			return gClient.channels.getLength()
		},
		func() fyne.CanvasObject {
			return container.NewVBox(widget.NewButton("", func() {}))
		},
		func(i widget.ListItemID, item fyne.CanvasObject) {
			channel := gClient.channels.getChannels()[i]

			buttonName := item.(*fyne.Container).Objects[0].(*widget.Button)
			if channel.isFavorite {
				buttonName.Importance = widget.HighImportance
			} else {
				buttonName.Importance = widget.MediumImportance
			}

			buttonName.SetText(fmt.Sprintf("%s [%s]", channel.aliasName, cutHash384(channel.chanID)))
			buttonName.OnTapped = func() { setChatChanContent(ctx, w, channel) }

			buttonName.Refresh()
		},
	)

	mainContentVBox := container.NewBorder(nil, nil, nil, nil, channelsList)
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
		func() { setAddChannelContent(w) },
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
	mu               *sync.Mutex
	messages         map[string]struct{}
	startIndexReader *uint64
	w                fyne.Window
	filter           func(*models.MessageBody) bool
}

func newCustomScroller(content fyne.CanvasObject, startIndexReader *uint64, w fyne.Window) *customScroller {
	s := &customScroller{}
	s.Content = content
	s.mu = &sync.Mutex{}
	s.startIndexReader = startIndexReader
	s.messages = make(map[string]struct{}, 4096)
	s.w = w
	s.ExtendBaseWidget(s)
	s.OnScrolled = func(p fyne.Position) {
		s.mu.Lock()
		defer s.mu.Unlock()

		if s.Offset.Y > 0 {
			return
		}

		readUntil := int64(-1)
		if *s.startIndexReader > consts.CountMessagesPerPage {
			readUntil = int64(*s.startIndexReader - consts.CountMessagesPerPage)
		}

		index := int64(*s.startIndexReader)
		if readUntil >= 0 {
			*s.startIndexReader = uint64(readUntil)
		}

		if *s.startIndexReader == 0 && readUntil == -1 {
			index = -1
		}

		for index > readUntil {
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
			msgBody, err := gClient.decoder.MessageInfo(messageInfo, pubKey, currentChatChannel.pkHashes, currentChatChannel.key)
			if err != nil {
				fyne.Do(func() { dialog.ShowError(err, s.w) })
				return
			}
			index--
			if s.filter != nil && !s.filter(msgBody) {
				if readUntil > 0 {
					*s.startIndexReader--
				}
				if readUntil >= 0 {
					readUntil--
				}
				continue
			}
			fyne.Do(func() { addMessageToChat(s.w, s, pubKey, msgBody, true) })
		}
	}
	return s
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

type ratioLayout struct {
	ratios []float32
}

func (r *ratioLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(100, 38)
}

func (r *ratioLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	x := float32(0)
	for i, obj := range objects {
		if i >= len(r.ratios) {
			break
		}
		w := size.Width * r.ratios[i]
		obj.Resize(fyne.NewSize(w, size.Height))
		obj.Move(fyne.NewPos(x, 0))
		x += w
	}
}
