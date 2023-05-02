package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"golang.org/x/crypto/ssh"
)

type Config struct {
	Host     string `json:"host"`
	Username string `json:"username"`
	Password string `json:"password"`
	InfluxDB struct {
		Host     string `json:"host"`
		Token    string `json:"token"`
		Database string `json:"database"`
	} `json:"influxdb"`
}

func loadConfig(filename string) *Config {
	f, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	config := new(Config)
	err = json.NewDecoder(f).Decode(config)
	if err != nil {
		panic(err)
	}
	return config
}

func main() {
	var configFile string
	flag.StringVar(&configFile, "c", "config.json", "config file")
	flag.Parse()
	config := loadConfig(configFile)

	influxdb := influxdb2.NewClient(config.InfluxDB.Host, config.InfluxDB.Token)
	writeAPI := influxdb.WriteAPIBlocking("", config.InfluxDB.Database)

	sshConfig := &ssh.ClientConfig{
		User:              config.Username,
		Auth:              []ssh.AuthMethod{ssh.Password(config.Password)},
		HostKeyCallback:   ssh.InsecureIgnoreHostKey(),
		HostKeyAlgorithms: []string{"ssh-rsa"},
		ClientVersion:     "SSH-2.0-OpenSSH", // wtf APC???
	}

	client, err := ssh.Dial("tcp", config.Host, sshConfig)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		panic(err)
	}
	defer session.Close()

	stdinR, stdinW := io.Pipe()
	session.Stdin = stdinR

	stdoutR, stdoutW := io.Pipe()
	session.Stdout = stdoutW

	session.Stderr = io.Discard

	if err := session.Shell(); err != nil {
		panic(err)
	}

	go func() {
		for t := range time.NewTicker(1 * time.Second).C {
			_ = t
			stdinW.Write([]byte("phReading all current\n"))
		}
	}()

	scanner := bufio.NewScanner(stdoutR)
	for scanner.Scan() {
		t := scanner.Text()
		f := strings.Fields(t)
		if len(f) >= 2 && f[0] == "1:" {
			value, err := strconv.ParseFloat(f[1], 64)
			if err != nil {
				log.Println(err)
				continue
			}
			log.Println(value)
			p := influxdb2.NewPointWithMeasurement("current").
				AddField("_value", value).
				SetTime(time.Now())
			err = writeAPI.WritePoint(context.Background(), p)
			if err != nil {
				log.Printf("WritePoint: %v", err)
			}
		}
	}
}
