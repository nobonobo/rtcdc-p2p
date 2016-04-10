package client

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"
)

var BaseURL = "https://signaling-2016.appspot.com/"

type Request struct {
	Room string `json:"room"`
	ID   string `json:"id"`
}

type Members struct {
	Room    string   `json:"room"`
	Owner   string   `json:"owner"`
	Members []string `json:"members"`
}

type Message struct {
	Room    string          `json:"room"`
	Message json.RawMessage `json:"message,omitempty"`
	Last    uint64          `json:"last,omitempty"`
}

type Response struct {
	Room     string            `json:"room"`
	Messages []json.RawMessage `json:"messages"`
	Last     uint64            `json:"last"`
}

type Client struct {
	baseURL  string
	room     string
	id       string
	callback func(b []byte)
	send     chan []byte
	once     *sync.Once
	done     chan struct{}
	Members  *Members
	last     *uint64
}

// New ...
func New(room, id string, callback func(b []byte)) *Client {
	return &Client{
		baseURL:  BaseURL,
		room:     room,
		id:       id,
		callback: callback,
		send:     make(chan []byte, 100),
	}
}

// ID ...
func (c *Client) ID() string {
	return c.id
}

// Room ...
func (c *Client) Room() string {
	return c.room
}

// Create ...
func (c *Client) Create() error {
	return c.ctrl("create")
}

// Join ...
func (c *Client) Join() error {
	return c.ctrl("join")
}

// Bye ...
func (c *Client) Bye() error {
	return c.ctrl("bye")
}

// Start ...
func (c *Client) Start() {
	c.once = new(sync.Once)
	c.done = make(chan struct{})
	go c.run()
}

// Send ...
func (c *Client) Send(b []byte) {
	c.send <- b
}

// Stop ...
func (c *Client) Stop() {
	c.once.Do(func() { close(c.done) })
}

func (c *Client) ctrl(u string) error {
	b, err := json.Marshal(&Request{Room: c.room, ID: c.id})
	if err != nil {
		return err
	}
	resp, err := http.Post(c.baseURL+u, "application/json", bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if u == "bye" {
		io.Copy(ioutil.Discard, resp.Body)
		return nil
	}
	if resp.StatusCode == http.StatusOK {
		var members *Members
		if err := json.NewDecoder(resp.Body).Decode(&members); err != nil {
			return err
		}
		c.Members = members
		log.Println(members)
	}
	return nil
}

func (c *Client) comm(b []byte) error {
	last := uint64(0)
	if c.last != nil {
		last = *c.last
	}
	bf, err := json.Marshal(&Message{Room: c.room, Message: b, Last: last})
	if err != nil {
		return err
	}
	resp, err := http.Post(c.baseURL, "application/json", bytes.NewBuffer(bf))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var res *Response
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return err
	}
	if c.last == nil {
		c.last = new(uint64)
		*c.last = res.Last
		return nil
	}
	if len(res.Messages) > 0 {
		for _, b := range res.Messages {
			c.callback(b)
		}
	}
	/*
		if *c.last != res.Last {
			log.Println("last:", res.Last)
		}
	*/
	*c.last = res.Last
	return nil
}

func (c *Client) run() {
	log.Println("poller start")
	defer log.Println("poller stop")
	tm := time.NewTimer(time.Second)
	for {
		var b []byte
		select {
		case <-c.done:
			return
		case b = <-c.send:
		case <-tm.C:
		}
		tm.Reset(500 * time.Microsecond)
		if err := c.comm(b); err != nil {
			log.Println(err)
			//return
		}
	}
}
