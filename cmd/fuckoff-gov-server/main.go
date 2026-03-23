package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/number571/fuckoff-gov/internal/database"
)

var (
	addr string
	db   database.IDatabase
)

func init() {
	if len(os.Args) < 3 {
		panic("args < 3")
	}

	addr = os.Args[1]

	var err error
	db, err = database.OpenDatabase(os.Args[2])
	if err != nil {
		panic(err)
	}
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/ping", handlePing)

	mux.HandleFunc("/client/init", handleClientInit)
	mux.HandleFunc("/client/load", handleClientLoad)

	mux.HandleFunc("/client/channels/size", handleClientChannelsSize)
	mux.HandleFunc("/client/channels/listen", handleClientChannelsListen)

	mux.HandleFunc("/channel/init", handleChannelInit)
	mux.HandleFunc("/channel/load", handleChannelLoad)

	mux.HandleFunc("/channel/chat/push", handleChannelChatPush)
	mux.HandleFunc("/channel/chat/load", handleChannelChatLoad)
	mux.HandleFunc("/channel/chat/size", handleChannelChatSize)
	mux.HandleFunc("/channel/chat/listen", handleChannelChatListen)

	fmt.Println("service is listening...")
	http.ListenAndServe(addr, mux)
}
