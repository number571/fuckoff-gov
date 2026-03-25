package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"flag"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/number571/fuckoff-gov/internal/database/serverside"
)

var (
	db serverside.IServerDatabase
	sk []byte
)

var (
	certFile = "cert.pem"
	keyFile  = "key.pem"
)

var (
	certFilePath string
	keyFilePath  string
	databasePath string
	externalAddr string
	listenPort   string
)

func init() {
	flag.StringVar(&certFilePath, "cert", "cert.pem", "set path to certificate")
	flag.StringVar(&keyFilePath, "key", "key.pem", "set path to private key")
	flag.StringVar(&databasePath, "database", "server.db", "set path to database file")
	flag.StringVar(&externalAddr, "external-addr", "127.0.0.1", "set external address (domain or ip)")
	flag.StringVar(&listenPort, "listen-port", "9999", "set listening port of service")
	flag.Parse()
}

func init() {
	var err error
	db, err = serverside.OpenServerDatabase(databasePath)
	if err != nil {
		panic(err)
	}

	sk, err = db.GetSecretKey()
	if err != nil {
		panic(err)
	}

	if err := tryGenerateKeyFiles(externalAddr, listenPort); err != nil {
		panic(err)
	}

}

func tryGenerateKeyFiles(externalAddr, listenPort string) error {
	var (
		_, err1 = os.Stat(certFile)
		_, err2 = os.Stat(keyFile)
	)
	if err1 == nil && err2 == nil {
		return nil
	}
	if !os.IsNotExist(err1) || !os.IsNotExist(err2) {
		return errors.Join(err1, err2)
	}

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{Organization: []string{listenPort}},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(5, 0, 0), // Valid for 5 years
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	if ip := net.ParseIP(externalAddr); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	} else {
		template.DNSNames = append(template.DNSNames, externalAddr)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return err
	}

	certOut, err := os.Create(certFile)
	if err != nil {
		return err
	}
	defer certOut.Close()

	err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if err != nil {
		return err
	}

	keyOut, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer keyOut.Close()

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return err
	}

	err = pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})
	if err != nil {
		return err
	}

	return nil
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/ping", handlePing)
	mux.HandleFunc("/auth", handleAuth)

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

	server := &http.Server{
		Addr:    "0.0.0.0:" + listenPort,
		Handler: mux,
		TLSConfig: &tls.Config{
			MinVersion:               tls.VersionTLS13,
			PreferServerCipherSuites: true,
		},
		IdleTimeout:  2 * time.Minute,
		ReadTimeout:  time.Minute,
		WriteTimeout: time.Minute,
	}

	log.Printf("Service is listening on %s port\n", listenPort)
	log.Fatal(server.ListenAndServeTLS(certFile, keyFile))
}
