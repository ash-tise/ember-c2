package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/ash-tise/ember-c2/internal/proto"
)

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

func handleRegister(w http.ResponseWriter, r *http.Request) {

	var initBeacon proto.Beacon

	decoder := json.NewDecoder(r.Body)

	if err := decoder.Decode(&initBeacon); err != nil {
		log.Printf("ERROR: handleRegister: Failed to decode initBeacon JSON: %v", err)
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	log.Println("OPERATIONAL: Received registration attempt from Host:", initBeacon.Hostname)
}

func main() {
	initLogger()
}
