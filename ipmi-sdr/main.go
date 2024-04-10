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

type HostConfig struct {
	Name     string `json:"name"`
	Address  string `json:"address"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type Config struct {
	Hosts   []HostConfig `json:"hosts"`
	Sensors []string     `json:"sensors"`

	InfluxDB struct {
		Host     string `json:"host"`
		Token    string `json:"token"`
		Database string `json:"database"`
	} `json:"influxdb"`
}

func openIPMI(host HostConfig) (cmd *exec.Cmd) {
	if host.Address != "" {
		cmd = exec.Command("ipmitool",
			"-I", "lanplus",
			"-H", host.Address,
			"-U", host.Username,
			"-P", host.Password,
			"shell")
	} else {
		cmd = exec.Command("ipmitool", "shell")
	}
	return
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

func hostWorker(writeAPI api.WriteAPI, cmd *exec.Cmd, host string, sensors []string) {
	stdinW, _ := cmd.StdinPipe()
	stdoutR, _ := cmd.StdoutPipe()

	cmd.Start()
	defer cmd.Wait()

	b := []byte("sdr get " + strings.Join(sensors, " ") + "\n")
	stdinW.Write([]byte("set csv 1\n"))
	ticker := time.NewTicker(1 * time.Second)
	go func() {
		for t := range ticker.C {
			_ = t
			stdinW.Write(b)
		}
	}()
	defer ticker.Stop()

	scanner := bufio.NewScanner(stdoutR)
	for scanner.Scan() {
		t := scanner.Text()
		if strings.HasPrefix(t, "ipmitool>") {
			continue
		}
		f := strings.Split(t, ",")
		if len(f) < 2 {
			continue
		}
		value, err := strconv.ParseFloat(f[1], 64)
		if err != nil {
			log.Printf("%s %s: %v\n", host, f[0], err)
			continue
		}
		p := influxdb2.NewPointWithMeasurement("ipmi").
			AddTag("host", host).
			AddTag("sensor", f[0]).
			AddField("_value", value).
			SetTime(time.Now())
		writeAPI.WritePoint(p)
	}
}

func main() {
	var configFile string
	flag.StringVar(&configFile, "c", "config.json", "config file")
	flag.Parse()
	config := loadConfig(configFile)

	influxdb := influxdb2.NewClient(config.InfluxDB.Host, config.InfluxDB.Token)
	writeAPI := influxdb.WriteAPI("", config.InfluxDB.Database)

	for _, host := range config.Hosts {
		host := host
		go func() {
			for {
				cmd := openIPMI(host)
				hostWorker(writeAPI, cmd, host.Name, config.Sensors)
			}
		}()
	}
	<-make(chan struct{})
}
