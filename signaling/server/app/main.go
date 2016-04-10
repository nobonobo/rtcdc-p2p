package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/memcache"
)

// Expire ...
const Expire = time.Minute

// Request ...
type Request struct {
	Room string `json:"room"`
	ID   string `json:"id"`
}

// Members ...
type Members struct {
	Room     string   `json:"room"`
	Owner    string   `json:"owner"`
	Members  []string `json:"members"`
	ReadPos  uint64   `json:"readPos"`
	WritePos uint64   `json:"writePos"`
}

// Contain ...
func (m *Members) Contain(id string) (contain bool) {
	for _, m := range m.Members {
		if m == id {
			contain = true
			return
		}
	}
	return
}

// Remove ...
func (m *Members) Remove(id string) {
	for i, name := range m.Members {
		if name == id {
			m.Members = append(m.Members[:i], m.Members[i+1:]...)
			return
		}
	}
	return
}

// Message ...
type Message struct {
	Room    string          `json:"room"`
	Message json.RawMessage `json:"message,omitempty"`
	Last    uint64          `json:"last,omitempty"`
}

// Response ...
type Response struct {
	Room     string            `json:"room"`
	Messages []json.RawMessage `json:"messages"`
	Last     uint64            `json:"last"`
}

func writeHeader(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Add("Access-Control-Allow-Origin", origin)
		}
		w.Header().Add("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Add("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Add("Content-Type", "application/json;charset=UTF-8")
		if r.Method != "OPTIONS" {
			h.ServeHTTP(w, r)
		}
	})
}

func create(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	var req *Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Errorf(ctx, "req parse failed: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var members *Members
	_, err := memcache.JSON.Get(ctx, req.Room, &members)
	if err != nil || members.Owner != req.ID {
		members = &Members{
			Room:     req.Room,
			Owner:    req.ID,
			Members:  []string{},
			ReadPos:  0,
			WritePos: 0,
		}
	}
	item := &memcache.Item{Key: req.Room, Object: members, Expiration: 5 * Expire}
	if err := memcache.JSON.Set(ctx, item); err != nil {
		log.Errorf(ctx, "set failed: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(members); err != nil {
		log.Errorf(ctx, "res build failed: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
}

func join(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	var req *Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Errorf(ctx, "req parse failed: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var members *Members
	_, err := memcache.JSON.Get(ctx, req.Room, &members)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"error":"unknown room:%s" }`, req.Room)
		return
	}
	if !members.Contain(req.ID) {
		members.Members = append(members.Members, req.ID)
	}
	item := &memcache.Item{Key: req.Room, Object: members, Expiration: 5 * Expire}
	if err := memcache.JSON.Set(ctx, item); err != nil {
		log.Errorf(ctx, "set failed: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(members); err != nil {
		log.Errorf(ctx, "res build failed: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
}

func bye(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	var req *Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Errorf(ctx, "req parse failed: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var members *Members
	_, err := memcache.JSON.Get(ctx, req.Room, &members)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"error":"unknown room:%s" }`, req.Room)
		return
	}
	if req.ID == members.Owner {
		if err := memcache.Delete(ctx, req.Room); err != nil {
			log.Errorf(ctx, "del failed: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		members.Room = ""
	} else {
		if members.Contain(req.ID) {
			members.Remove(req.ID)
			item := &memcache.Item{Key: req.Room, Object: members, Expiration: 5 * Expire}
			if err := memcache.JSON.Set(ctx, item); err != nil {
				log.Errorf(ctx, "set failed: %s", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
	}
	if err := json.NewEncoder(w).Encode(members); err != nil {
		log.Errorf(ctx, "res build failed: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
}

func message(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	var msg *Message
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		log.Errorf(ctx, "msg parse failed: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var members *Members
	_, err := memcache.JSON.Get(ctx, msg.Room, &members)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"error":"unknown room:%s" }`, msg.Room)
		return
	}
	if len(msg.Message) > 0 {
		members.WritePos++
		key := fmt.Sprintf("%s-%d", msg.Room, members.WritePos)
		msgItem := &memcache.Item{Key: key, Value: msg.Message, Expiration: Expire}
		b, err := json.Marshal(members)
		if err != nil {
			log.Errorf(ctx, "encode failed: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		memItem := &memcache.Item{Key: msg.Room, Value: b, Expiration: Expire}
		if err := memcache.SetMulti(ctx, []*memcache.Item{memItem, msgItem}); err != nil {
			log.Errorf(ctx, "set failed: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	begin := members.ReadPos
	if begin < msg.Last {
		begin = msg.Last
	}
	res := &Response{
		Room:     msg.Room,
		Messages: []json.RawMessage{},
	}
	for i := begin + 1; i <= members.WritePos; i++ {
		item, err := memcache.Get(ctx, fmt.Sprintf("%s-%d", msg.Room, i))
		if err != nil {
			members.ReadPos = i
			continue
		}
		res.Messages = append(res.Messages, json.RawMessage(item.Value))
	}
	item := &memcache.Item{Key: msg.Room, Object: members, Expiration: Expire}
	if err := memcache.JSON.Set(ctx, item); err != nil {
		log.Errorf(ctx, "set failed: %s", err)
	}
	res.Last = members.WritePos
	if err := json.NewEncoder(w).Encode(res); err != nil {
		log.Errorf(ctx, "res build failed: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
}

func init() {
	mux := http.NewServeMux()
	mux.HandleFunc("/create", create)
	mux.HandleFunc("/join", join)
	mux.HandleFunc("/bye", bye)
	mux.HandleFunc("/", message)
	http.Handle("/", writeHeader(mux))
}
