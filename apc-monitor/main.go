package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"time"

	"github.com/gosnmp/gosnmp"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
)

type Config struct {
	Host     string `json:"host"`
	OID      string `json:"oid"`
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

	snmp := new(gosnmp.GoSNMP)
	*snmp = *gosnmp.Default
	snmp.Target = config.Host
	if err := snmp.Connect(); err != nil {
		panic(err)
	}

	for t := range time.NewTicker(1 * time.Second).C {
		_ = t
		result, err := snmp.Get([]string{config.OID})
		if err != nil {
			log.Println(err)
			continue
		}
		value := float64(result.Variables[0].Value.(uint)) / 10.0
		p := influxdb2.NewPointWithMeasurement("current").
			AddField("_value", value).
			SetTime(time.Now())
		err = writeAPI.WritePoint(context.Background(), p)
		if err != nil {
			log.Printf("WritePoint: %v", err)
		}
	}
}
