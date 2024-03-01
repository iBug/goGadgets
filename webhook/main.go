package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

type GitPullPayload struct {
	Ref string `json:"ref"`
}

var (
	listenPort string
	workDir    string
	urlPath    string
	branch     string
)

func HandleGitPull(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		log.Printf("io.ReadAll failed: %s\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if keystring, ok := os.LookupEnv("WEBHOOK_SECRET"); ok {
		sigStr := req.Header.Get("X-Hub-Signature")
		sig, ok := strings.CutPrefix(sigStr, "sha1=")
		if !ok {
			log.Printf("Missing signature\n")
			http.Error(w, "Missing signature\n", http.StatusForbidden)
			return
		}
		sigmac, err := hex.DecodeString(sig)
		if err != nil {
			log.Printf("Invalid signature: %s\n", err)
			http.Error(w, "Invalid signature\n", http.StatusForbidden)
			return
		}
		mac := hmac.New(sha1.New, []byte(keystring))
		mac.Write(body)
		if !hmac.Equal(sigmac, mac.Sum(nil)) {
			log.Printf("Bad signature: Expected %x, got %x\n", mac.Sum(nil), sigmac)
			http.Error(w, "Bad signature\n", http.StatusForbidden)
			return
		}
	}

	var payload GitPullPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("Invalid JSON: %s\n", err)
		http.Error(w, "Invalid JSON\n", http.StatusBadRequest)
		return
	}
	if payload.Ref != "refs/heads/"+branch {
		log.Printf("Ignoring ref %s\n", payload.Ref)
		http.Error(w, "Not interested in this ref\n", http.StatusOK)
		return
	}

	cmd := exec.Command("/bin/sh", "-c", "git fetch origin "+branch+" && git reset --hard FETCH_HEAD")
	cmd.Dir = workDir
	if err := cmd.Start(); err != nil {
		log.Printf("exec.Command failed: %s\n", err)
		http.Error(w, "Webhook failed\n", http.StatusInternalServerError)
		return
	}
	go cmd.Wait()
	http.Error(w, "OK\n", http.StatusOK)
}

func main() {
	flag.StringVar(&listenPort, "l", "127.0.0.1:8001", "listen address and port")
	flag.StringVar(&workDir, "c", "/var/www/html", "git repo location")
	flag.StringVar(&urlPath, "p", "/webhook/github/pull", "url path")
	flag.StringVar(&branch, "b", "gh-pages", "deployment branch")
	flag.Parse()
	// $JOURNAL_STREAM is set by systemd v231+
	if _, ok := os.LookupEnv("JOURNAL_STREAM"); ok {
		log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))
	}

	http.HandleFunc(urlPath, HandleGitPull)
	log.Fatal(http.ListenAndServe(listenPort, nil))
}
