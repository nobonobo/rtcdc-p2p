package datachan

import (
	"io"
	"log"
	"net"
	"time"

	webrtc "github.com/keroserene/go-webrtc"
)

type connWrapper struct {
	*webrtc.DataChannel
	dst io.Reader
}

func NewConn(channel *webrtc.DataChannel) net.Conn {
	dst, src := io.Pipe()
	channel.OnOpen = func() {
		log.Println("DataChannel Open")
	}
	channel.OnClose = func() {
		log.Println("DataChannel Close")
	}
	channel.OnMessage = func(b []byte) {
		_, err := src.Write(b)
		if err != nil {
			log.Println("write error:", err)
		}
	}
	return &connWrapper{
		DataChannel: channel,
		dst:         dst,
	}
}

func (w *connWrapper) Read(b []byte) (int, error) {
	return w.dst.Read(b)
}

func (w *connWrapper) Write(b []byte) (int, error) {
	w.DataChannel.Send(b)
	return len(b), nil
}

func (w *connWrapper) LocalAddr() net.Addr                { return nil }
func (w *connWrapper) RemoteAddr() net.Addr               { return nil }
func (w *connWrapper) SetDeadline(t time.Time) error      { return nil }
func (w *connWrapper) SetReadDeadline(t time.Time) error  { return nil }
func (w *connWrapper) SetWriteDeadline(t time.Time) error { return nil }

// Connection ...
type Connection struct {
	*webrtc.PeerConnection
	ices []webrtc.IceCandidate
}

// New ...
func New(iceServers []string) (*Connection, error) {
	config := webrtc.NewConfiguration(webrtc.OptionIceServer(iceServers...))
	pc, err := webrtc.NewPeerConnection(config)
	if nil != err {
		return nil, err
	}
	return &Connection{
		PeerConnection: pc,
	}, nil
}

// IceCandidates ...
func (pc *Connection) IceCandidates() []webrtc.IceCandidate {
	return pc.ices
}

//Offer ...
func (pc *Connection) Offer() (*webrtc.SessionDescription, error) {
	ices := []webrtc.IceCandidate{}
	pc.OnIceCandidate = func(ice webrtc.IceCandidate) {
		ices = append(ices, ice)
	}
	done := make(chan struct{})
	pc.OnIceComplete = func() {
		log.Println("OnIceComplete")
		close(done)
	}
	pc.OnConnectionStateChange = func(s webrtc.PeerConnectionState) {
		log.Println("OnConnectionStateChange:", s)
	}
	pc.OnIceCandidateError = func() {
		log.Println("OnIceCandidateError")
	}
	need := make(chan struct{})
	pc.OnNegotiationNeeded = func() {
		close(need)
	}
	dc, err := pc.CreateDataChannel("test", webrtc.Init{})
	if err != nil {
		return nil, err
	}
	<-need
	offer, err := pc.CreateOffer()
	if err != nil {
		return nil, err
	}
	pc.SetLocalDescription(offer)
	<-done
	pc.ices = ices
	if pc.OnDataChannel != nil {
		pc.OnDataChannel(dc)
	}
	return offer, nil
}

// Answer ...
func (pc *Connection) Answer(remote *webrtc.SessionDescription) (*webrtc.SessionDescription, error) {
	ices := []webrtc.IceCandidate{}
	pc.OnIceCandidate = func(ice webrtc.IceCandidate) {
		ices = append(ices, ice)
	}
	done := make(chan struct{})
	pc.OnIceComplete = func() {
		log.Println("OnIceComplete")
		close(done)
	}
	pc.OnConnectionStateChange = func(s webrtc.PeerConnectionState) {
		log.Println("OnConnectionStateChange:", s)
	}
	pc.OnIceCandidateError = func() {
		log.Println("OnIceCandidateError")
	}
	if err := pc.SetRemoteDescription(remote); err != nil {
		return nil, err
	}
	answer, err := pc.CreateAnswer()
	if err != nil {
		return nil, err
	}
	pc.SetLocalDescription(answer)
	<-done
	pc.ices = ices
	return answer, nil
}
