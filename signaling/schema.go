package signaling

import (
	"encoding/json"
	"fmt"
)

type Message struct {
	Type   string          `json:"type"`
	Sender string          `json:"sender"`
	To     string          `json:"to"`
	Value  json.RawMessage `json:"value"`
}

func New(sender, to string, value interface{}) *Message {
	var tp string
	switch value.(type) {
	case *Request:
		tp = "request"
	case *Offer:
		tp = "offer"
	case *Answer:
		tp = "answer"
	case *Candidate:
		tp = "candidate"
	default:
		panic("unknown type")
	}
	b, _ := json.Marshal(value)
	return &Message{
		Type:   tp,
		Sender: sender,
		To:     to,
		Value:  b,
	}
}

func (m *Message) Get() (interface{}, error) {
	var value interface{}
	switch m.Type {
	case "request":
		value = new(Request)
	case "offer":
		value = new(Offer)
	case "answer":
		value = new(Answer)
	case "candidate":
		value = new(Candidate)
	default:
		return nil, fmt.Errorf("unknown type: %s", m.Type)
	}
	if err := json.Unmarshal(m.Value, &value); err != nil {
		return nil, err
	}
	return value, nil
}

type Request struct{}

type Offer struct{ Description string }
type Answer struct{ Description string }

type Candidate struct {
	Candidate     string `json:"candidate"`
	SdpMLineIndex int    `json:"sdpMLineIndex"`
	SdpMid        string `json:"sdpMid"`
}
