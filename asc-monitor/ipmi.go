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

type HostConfig struct {
	Name     string `json:"name"`
	Address  string `json:"address"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type IPMIConfig struct {
	Hosts   []HostConfig `json:"hosts"`
	Sensors []string     `json:"sensors"`
}

type IPMIMonitor struct{}

func (IPMIMonitor) openCmd(host HostConfig) (cmd *exec.Cmd) {
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

func (IPMIMonitor) worker(writeAPI api.WriteAPI, cmd *exec.Cmd, host string, sensors []string) error {
	stdinW, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdoutR, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

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
	return cmd.Wait()
}

func (i IPMIMonitor) workerLoop(writeAPI api.WriteAPI, host HostConfig, sensors []string) {
	for {
		cmd := i.openCmd(host)
		i.worker(writeAPI, cmd, host.Name, sensors)
	}
}

func (i IPMIMonitor) Start(writeAPI api.WriteAPI, config IPMIConfig) error {
	for _, host := range config.Hosts {
		go i.workerLoop(writeAPI, host, config.Sensors)
	}
	return nil
}
