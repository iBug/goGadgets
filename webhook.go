package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

type WebhookPayload struct {
	Ref string `json:"ref"`
}

func HandleWebhook(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if keystring, ok := os.LookupEnv("WEBHOOK_SECRET"); ok {
		sig := req.Header.Get("X-Hub-Signature")
		if len(sig) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Missing signature\n")
			return
		}
		if !strings.HasPrefix(sig, "sha1=") {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, "Invalid signature\n")
			return
		}
		sigmac, err := hex.DecodeString(sig[5:])
		if err != nil {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, "Invalid signature\n")
			return
		}
		key := []byte(keystring)
		mac := hmac.New(sha1.New, key)
		mac.Write(body)
		if !hmac.Equal(sigmac, mac.Sum(nil)) {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, "Bad signature\n")
			return
		}
	}

	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Invalid JSON\n")
		return
	}
	if payload.Ref == "refs/heads/gh-pages" {
		cmd := exec.Command("sh", "-c", "cd /var/www/html; git fetch origin gh-pages; git reset --hard FETCH_HEAD")
		if err := cmd.Start(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Webhook failed\n")
		} else {
			fmt.Fprintf(w, "OK\n")
		}
	} else {
		fmt.Fprintf(w, "Not interested in this ref\n")
	}
}

func main() {
	http.HandleFunc("/webhook/github/pull", HandleWebhook)
	log.Fatal(http.ListenAndServe("127.0.0.1:8000", nil))
}
