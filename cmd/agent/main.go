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
	"runtime"
	"time"

	"github.com/ash-tise/ember-c2/internal/proto"
)

var c2URL string = "https://localhost:8443"
var client *http.Client
var agentID string
var generator *rand.Rand

func initClient() {
	// set up config for client creation
	config := &tls.Config{InsecureSkipVerify: true}
	transport := &http.Transport{TLSClientConfig: config}
	client = &http.Client{Transport: transport}
}

func register() error {
	// get Agent Hostname and OS
	host, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("Unable to fetch Hostname: %v", err)
	}
	osName := runtime.GOOS

	// place data into Beacon struct
	beacon := proto.Beacon{
		Hostname: host,
		OS:       osName,
	}

	// prepare and send beacon data to C2
	data, err := json.Marshal(&beacon)
	if err != nil {
		return fmt.Errorf("Failure encoding beacon data: %v", err)
	}
	body := bytes.NewReader(data)
	request, err := http.NewRequest("POST", c2URL, body)
	if err != nil {
		return fmt.Errorf("Error building POST request: %v", err)
	}
	request.Header.Add("Content-Type", "application/json")

	// get HTTP response from server
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("Failure getting HTTP response: %v", err)
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
	// make buffer to randomize an int64
	buff := make([]byte, 8)
	if _, err := cryptorand.Read(buff); err != nil {
		log.Fatalf("Error generating random seed: %v", err)
	}
	seed := binary.LittleEndian.Uint64(buff)
	generator = rand.New(rand.NewSource(int64(seed)))
}

func main() {
	// initialization functions
	initRand()
	initClient()
	if err := register(); err != nil {
		log.Fatalf("Failed to register agent: %v", err)
	}

	// beacon sleep range in seconds
	minSec := 30
	maxSec := 100

	for {
		timeToSleep := generator.Intn(maxSec-minSec) + 30
		time.Sleep(time.Duration(timeToSleep) * time.Second)
		// add beacon() here
	}
}
