package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/ash-tise/ember-c2/internal/proto"
)

var activeAgents = make(map[string]*proto.AgentMetadata)
var agentMutex sync.Mutex

func initLogger() {
	// create or append to logfile
	logFile, err := os.OpenFile("ember.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("FATAL: Could not open log file: %v", err)
	}
	// initialize multiwriter for logging to stdout and file
	multiwriter := io.MultiWriter(os.Stdout, logFile)

	// configure logging options
	log.SetOutput(multiwriter)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	log.Println("Ember Logger Initialized. Writing to ember.log and Console.")
}

func initConsole() {
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	// grab beacon info from agent
	var initBeacon proto.Beacon
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&initBeacon); err != nil {
		log.Printf("ERROR: handleRegister: Failed to decode initBeacon JSON: %v", err)
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	log.Println("OPERATIONAL: Received registration attempt from Host:", initBeacon.Hostname)

	// generate random agentID and send back to the agent
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		log.Printf("FATAL INTERNAL ERROR: crypto/rand failure: %v", err)
		http.Error(w, "Service Unavailable", http.StatusInternalServerError)
		return
	}
	initBeacon.AgentID = hex.EncodeToString(buffer)

	// store new agent information on the server
	newAgent := proto.AgentMetadata{
		AgentID:      initBeacon.AgentID,
		Hostname:     initBeacon.Hostname,
		OS:           initBeacon.OS,
		Arch:         initBeacon.Arch,
		LastCheckIn:  time.Now(),
		CommandQueue: make(chan proto.Command, 10)}

	agentMutex.Lock()
	defer agentMutex.Unlock()
	activeAgents[initBeacon.AgentID] = &newAgent

	// return agentID to agent
	w.Write([]byte(initBeacon.AgentID))
}

func handleBeacon(w http.ResponseWriter, r *http.Request) {
}

func main() {
	// initialize logger and console
	initLogger()
	go initConsole()

	// initialize server
	mux := http.NewServeMux()
	mux.HandleFunc("/register", handleRegister)
	mux.HandleFunc("/beacon", handleBeacon)
	log.Fatal(http.ListenAndServeTLS("localhost:8443", "server.crt", "server.key", mux))
}
