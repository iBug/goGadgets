package main

import (
	"bufio"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
)

var sshCommand = []string{"ssh", "-o", "StrictHostKeyChecking=no", "-T"}

type SSHHost struct {
	Name    string `json:"name"`
	SSHName string `json:"sshname"`
}

type NVIDIAConfig struct {
	Hosts []SSHHost `json:"hosts"`
}

type NVIDIAMonitor struct{}

func (NVIDIAMonitor) worker(writeAPI api.WriteAPI, host SSHHost) error {
	hostname := host.Name

	cmdline := []string{"nvidia-smi", "dmon", "-s", "pm"}
	if host.SSHName != "" {
		sshCmdline := append(sshCommand, host.SSHName)
		cmdline = append(sshCmdline, cmdline...)
	}
	cmd := exec.Command(cmdline[0], cmdline[1:]...)
	stdoutR, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
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

func (n NVIDIAMonitor) workerLoop(writeAPI api.WriteAPI, host SSHHost) {
	for {
		log.Println(n.worker(writeAPI, host))
	}
}

func (n NVIDIAMonitor) Start(writeAPI api.WriteAPI, config NVIDIAConfig) {
	for _, host := range config.Hosts {
		go n.workerLoop(writeAPI, host)
	}
}
