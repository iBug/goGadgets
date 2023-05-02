// while true; do echo 'set csv 1'; while true; do echo 'sdr get PSU1_PIN PSU1_POUT CPU_Power FAN_Power Memory_Power Total_Power'; sleep 1; done; done | sudo ipmitool shell | grep Watts

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Config struct {
	Host struct {
		Address  string `json:"address"`
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"host"`
	Sensors []string `json:"sensors"`
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
		fmt.Printf("%s: %s\n", f[0], f[1])
	}
}
