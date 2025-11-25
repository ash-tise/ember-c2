package proto

import "time"

type Beacon struct {
	AgentID  string `json:"id"`
	Hostname string `json:"host"`
	OS       string `json:"os"`
	Result   string `json:"r"`
}

type Command struct {
	TaskID    string `json:"tid"`
	Action    string `json:"act"`
	Arguments string `json:"args"`
}

type Response struct {
	Commands []Command `json:"cmds"`
}

type AgentMetadata struct {
	AgentID      string
	Hostname     string
	OS           string
	LastCheckIn  time.Time
	CommandQueue chan Command
}
