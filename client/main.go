package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/rpc/jsonrpc"
	"time"

	webrtc "github.com/keroserene/go-webrtc"
	"github.com/nobonobo/rtcdc-p2p/datachan"
	"github.com/nobonobo/rtcdc-p2p/signaling"
	"github.com/nobonobo/rtcdc-p2p/signaling/client"
)

var iceServers = []string{"stun:stun.l.google.com:19302"}

//var iceServers = []string{}

// Client ...
type Client struct {
	*client.Client
	conn *datachan.Connection
}

// New ...
func New(room, id string) *Client {
	c := new(Client)
	c.Client = client.New(room, id, c.dispatch)
	return c
}

// Open ...
func (c *Client) Open() (net.Conn, error) {
	con, err := datachan.New(iceServers)
	if err != nil {
		return nil, err
	}
	c.conn = con
	if err := c.Join(); err != nil {
		return nil, err
	}
	defer c.Bye()
	complete := make(chan net.Conn, 1)
	c.conn.OnDataChannel = func(channel *webrtc.DataChannel) {
		complete <- datachan.NewConn(channel)
	}
	c.Start()
	defer c.Stop()
	time.Sleep(time.Second)
	if err := c.Send("", &signaling.Request{}); err != nil {
		return nil, err
	}
	channel := <-complete
	return channel, nil
}

// Close ...
func (c *Client) Close() {

}

// Send ...
func (c *Client) Send(to string, v interface{}) error {
	log.Println("send to:", to, v)
	m := signaling.New(c.ID(), to, v)
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	c.Client.Send(b)
	return nil
}

func (c *Client) dispatch(b []byte) {
	var m *signaling.Message
	if err := json.Unmarshal(b, &m); err != nil {
		log.Println(err)
		return
	}
	if m.Sender == c.ID() {
		return
	}
	if m.To != c.ID() {
		return
	}
	value, err := m.Get()
	if err != nil {
		log.Println(err)
		return
	}
	log.Printf("recv: %T from %s\n", value, m.Sender)
	switch v := value.(type) {
	case *signaling.Request:
	case *signaling.Offer:
		sdp := &webrtc.SessionDescription{
			Type: "offer",
			Sdp:  v.Description,
		}
		answer, err := c.conn.Answer(sdp)
		if err != nil {
			log.Println(err)
			return
		}
		if err := c.Send(m.Sender, &signaling.Answer{Description: answer.Sdp}); err != nil {
			log.Println(err)
			return
		}
		ices := c.conn.IceCandidates()
		log.Println("ices:", len(ices))
		for _, ice := range ices[2:] {
			msg := &signaling.Candidate{
				Candidate:     ice.Candidate,
				SdpMid:        ice.SdpMid,
				SdpMLineIndex: ice.SdpMLineIndex,
			}
			log.Printf("candidate: %q\n", ice.Candidate)
			if err := c.Send(m.Sender, msg); err != nil {
				log.Println(err)
				return
			}
			time.Sleep(100 * time.Microsecond)
		}
	case *signaling.Answer:
	case *signaling.Candidate:
		ice := webrtc.DeserializeIceCandidate(string(m.Value))
		if err := c.conn.AddIceCandidate(*ice); err != nil {
			log.Println(err)
		}
	}
}

func main() {
	var room, id string
	flag.StringVar(&room, "room", "sample", "name of room")
	flag.StringVar(&id, "id", "", "name of id")
	flag.Parse()
	if id == "" {
		log.Fatalln("id must set unique")
	}
	webrtc.SetLoggingVerbosity(0)
	c := New(room, id)
	conn, err := c.Open()
	if err != nil {
		log.Fatalln(err)
	}
	defer c.Close()
	defer conn.Close()
	client := jsonrpc.NewClient(conn)
	log.Println("completed:", client)
	var reply string
	begin := time.Now()
	for i := 0; i < 1000; i++ {
		if err := client.Call("Service.Echo", "hello!", &reply); err != nil {
			log.Println(err)
		}
	}
	fmt.Println("Call 1000times:", time.Since(begin))
	fmt.Println("       average:", time.Duration(int64(time.Since(begin))/1000))
	begin = time.Now()
	for i := 0; i < 1000; i++ {
		if call := client.Go("Service.Echo", "hello!", &reply, nil); call.Error != nil {
			log.Println(err)
		}
	}
	fmt.Println("Go   1000times:", time.Since(begin))
	fmt.Printf("    throughput: %dr/s\n", int64(1000*time.Second)/int64(time.Since(begin)))
}
