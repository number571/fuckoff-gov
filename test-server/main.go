package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/apk", func(w http.ResponseWriter, r *http.Request) {
		filePath := "./test-client/fyne-cross/dist/android-arm64/test-client.apk"
		// filePath := "./fyne-cross/dist/android-arm64/fuckoff-gov.apk"
		fileName := "test-client.apk"

		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
		w.Header().Set("Content-Type", "application/octet-stream")

		http.ServeFile(w, r, filePath)
	})
	http.HandleFunc("/cert", func(w http.ResponseWriter, r *http.Request) {
		filePath := "./cert.pem"
		fileName := "cert.pem"

		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
		w.Header().Set("Content-Type", "application/octet-stream")

		http.ServeFile(w, r, filePath)
	})
	fmt.Println("Test server is listening...")
	http.ListenAndServe("0.0.0.0:8081", nil)
}
