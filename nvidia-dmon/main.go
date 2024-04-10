package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
)

type Config struct {
	InfluxDB struct {
		Host     string `json:"host"`
		Token    string `json:"token"`
		Database string `json:"database"`
	} `json:"influxdb"`
}

var hostname string

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

func run(writeAPI api.WriteAPI) error {
	cmd := exec.Command("nvidia-smi", "dmon", "-s", "pm")
	stdoutR, _ := cmd.StdoutPipe()
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		panic(err)
	}

	scanner := bufio.NewScanner(stdoutR)
	for scanner.Scan() {
		t := scanner.Text()
		if strings.HasPrefix(t, "#") {
			continue
		}
		f := strings.Fields(t)
		if len(f) != 6 {
			continue
		}
		value1, err := strconv.Atoi(f[1])
		if err != nil {
			log.Println(err)
			continue
		}
		value2, err := strconv.Atoi(f[2])
		if err != nil {
			log.Println(err)
			continue
		}
		value4, err := strconv.Atoi(f[4])
		if err != nil {
			log.Println(err)
			continue
		}
		p := influxdb2.NewPointWithMeasurement("nvidia").
			AddTag("host", hostname).
			AddTag("id", f[0]).
			AddField("pwr", value1).
			AddField("gtemp", value2).
			AddField("fb", value4).
			SetTime(time.Now())
		writeAPI.WritePoint(p)
	}
	return cmd.Wait()
}

func main() {
	var configFile string
	flag.StringVar(&configFile, "c", "config.json", "config file")
	flag.Parse()
	config := loadConfig(configFile)
	hostname, _ = os.Hostname()

	influxdb := influxdb2.NewClient(config.InfluxDB.Host, config.InfluxDB.Token)
	writeAPI := influxdb.WriteAPI("", config.InfluxDB.Database)

	for {
		run(writeAPI)
	}
}
