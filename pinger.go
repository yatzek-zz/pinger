package main

import (
	"os/exec"
	"bytes"
	"strings"
	"time"
	"os"
	"os/signal"
	"syscall"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"fmt"
)

const intervalSeconds = 30

func main() {

	argsWithoutProg := os.Args[1:]
	if len(argsWithoutProg) != 1 {
		log.Fatal("Missing config file name param")
	}
	config := parseConfig(argsWithoutProg[0])

	pingsChan := make(chan pingStatus)
	signalChan := make(chan os.Signal, 1)

	periodicallyRunPings(config, pingsChan)
	printPings(pingsChan)

	signal.Notify(signalChan, syscall.SIGTERM, syscall.SIGQUIT)
	<-signalChan
	log.Println("Got signal, exitting!")
}

func periodicallyRunPings(config config, pingsChan chan pingStatus) {

	ticker := time.NewTicker(intervalSeconds * time.Second)

	go func() {
		for {
			select {
			case <-ticker.C:
				fmt.Println("Triggering pings...")
				pingAll(config, pingsChan)
			}
		}
	}()
}

func printPings(pingsChan chan pingStatus) {
	go func() {
		for {
			select {
			case ping := <-pingsChan:
				fmt.Println(ping.String())
			}
		}
	}()
}

func pingAll(config config, pingsChan chan pingStatus) {
	for _, host := range config.Hosts {
		go fpingsFromHost(config.Username, host, config.Hosts, pingsChan)
	}
}

func fpingsFromHost(username string, fromHost string, toHosts []string, pingsChan chan pingStatus) {

	// TODO: remove self (fromHost) from hosts ??? - keeping for now, perhaps we see errors in such combinations???

	var out bytes.Buffer
	cmd := exec.Command("ssh", username+"@"+fromHost, "fping -C 1 -q "+strings.Join(toHosts, " ")+" || true")
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()

	if err != nil {
		//log.Printf(">>> ERROR SSHING TO HOST: %s, ERROR: %s\n", fromHost, err)
		pingsChan <- pingStatus{From: fromHost, SshError: true}
		return
	}

	for _, ping := range parsePingStatus(fromHost, out.String()) {
		pingsChan <- ping
	}
}

type pingStatus struct {
	From      string
	To        string
	SshError  bool
	PingError bool
}

func (p pingStatus) String() string {
	if p.SshError {
		return p.From + ": ERROR_SSHING_TO_HOST"
	}
	status := "OK"
	if p.PingError {
		status = "FAILED"
	}
	return p.From + " => " + p.To + ": " + status
}

func parsePingStatus(fromHost, output string) (statuses []pingStatus) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		tokens := strings.Split(line, " : ")
		toIP := tokens[0]
		pingTime := tokens[1]
		pingError := false
		if pingTime == "-" {
			pingError = true
		}
		statuses = append(statuses, pingStatus{From: fromHost, To: toIP, PingError: pingError})
	}
	return statuses
}

type config struct {
	Username string   `yaml:"username"`
	Hosts    []string `yaml:"hosts"`
}

func parseConfig(file string) config {
	config := config{}
	data, err := ioutil.ReadFile(file)
	if err != nil {
		log.Fatalf("error parsing config: %v", err)
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Fatalf("error parsing config: %v", err)
	}

	return config
}

// REQUIREMENTS
// 1. setup password-less ssh
// 2. run: fping -C 1 -q 172.17.0.6 172.17.0.7 172.17.0.8
// 3. capture output, record it, present it

//❯ fping -C 1 -q 172.17.0.3 172.17.0.5 172.17.0.8
//172.17.0.6 : 0.36
//172.17.0.7 : 0.48
//172.17.0.8 : -
