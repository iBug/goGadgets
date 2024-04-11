package main

import (
	"log"
	"time"

	"github.com/gosnmp/gosnmp"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
)

type APCConfig struct {
	Host string `json:"host"`
	OID  string `json:"oid"`
}

type APCMonitor struct{}

func (APCMonitor) worker(writeAPI api.WriteAPI, config APCConfig) error {
	snmp := new(gosnmp.GoSNMP)
	*snmp = *gosnmp.Default
	snmp.Target = config.Host
	if err := snmp.Connect(); err != nil {
		return err
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
		writeAPI.WritePoint(p)
	}
	return nil
}

func (a APCMonitor) workerLoop(writeAPI api.WriteAPI, config APCConfig) {
	for {
		log.Println(a.worker(writeAPI, config))
	}
}

func (a APCMonitor) Start(writeAPI api.WriteAPI, config APCConfig) {
	go a.workerLoop(writeAPI, config)
}
