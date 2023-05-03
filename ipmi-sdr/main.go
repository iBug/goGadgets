package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
)

type Config struct {
	Hostname string `json:"hostname"`

	Host struct {
		Address  string `json:"address"`
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"host"`
	Sensors []string `json:"sensors"`

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

	b := []byte("sdr get " + strings.Join(config.Sensors, " ") + "\n")

	var cmd *exec.Cmd
	if config.Host.Address != "" {
		cmd = exec.Command("ipmitool",
			"-I", "lanplus",
			"-H", config.Host.Address,
			"-U", config.Host.Username,
			"-P", config.Host.Password,
			"shell")
	} else {
		cmd = exec.Command("ipmitool", "shell")
	}
	stdinW, _ := cmd.StdinPipe()
	stdoutR, _ := cmd.StdoutPipe()

	cmd.Start()
	defer cmd.Wait()

	stdinW.Write([]byte("set csv 1\n"))
	go func() {
		for t := range time.NewTicker(time.Second).C {
			_ = t
			stdinW.Write(b)
		}
	}()

	scanner := bufio.NewScanner(stdoutR)
	for scanner.Scan() {
		t := scanner.Text()
		if strings.HasPrefix(t, "ipmitool>") {
			fmt.Println()
			fmt.Println(time.Now().Format("2006-01-02 15:04:05"))
			continue
		}
		f := strings.Split(t, ",")
		if len(f) < 2 {
			continue
		}
		value, err := strconv.ParseFloat(f[1], 64)
		if err != nil {
			log.Println(err)
			continue
		}
		p := influxdb2.NewPointWithMeasurement("ipmi").
			AddTag("host", config.Hostname).
			AddTag("sensor", f[0]).
			AddField("_value", value).
			SetTime(time.Now())
		err = writeAPI.WritePoint(context.Background(), p)
		if err != nil {
			log.Printf("WritePoint: %v", err)
		}
	}
}
