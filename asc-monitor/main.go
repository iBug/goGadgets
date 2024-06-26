package main

import (
	"bufio"
	"flag"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"sigs.k8s.io/yaml"
)

type InfluxDBConfig struct {
	Host     string `json:"host"`
	Token    string `json:"token"`
	Database string `json:"database"`
}

type Config struct {
	IPMI   *IPMIConfig   `json:"ipmi-sdr"`
	NVIDIA *NVIDIAConfig `json:"nvidia-smi"`
	APC    *APCConfig    `json:"apc"`

	InfluxDB InfluxDBConfig `json:"influxdb"`
}

func loadConfig(filename string) *Config {
	b, err := os.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	config := new(Config)
	err = yaml.Unmarshal(b, &config)
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
	flag.StringVar(&configFile, "c", "config.yml", "config file")
	flag.Parse()
	config := loadConfig(configFile)

	if _, ok := os.LookupEnv("JOURNAL_STREAM"); ok {
		log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))
	}

	influxdb := influxdb2.NewClient(config.InfluxDB.Host, config.InfluxDB.Token)
	writeAPI := influxdb.WriteAPI("", config.InfluxDB.Database)

	if config.IPMI != nil {
		IPMIMonitor{}.Start(writeAPI, *config.IPMI)
	}
	if config.NVIDIA != nil {
		NVIDIAMonitor{}.Start(writeAPI, *config.NVIDIA)
	}
	if config.APC != nil {
		APCMonitor{}.Start(writeAPI, *config.APC)
	}

	<-make(chan struct{})
}
