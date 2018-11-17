package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
)

const FACEBOOK_API = "https://graph.facebook.com/v2.6/me/messages?access_token=%s"

type Callback struct {
	Object string `json:"object,omitempty"`
	Entry  []struct {
		ID     string  `json:"id,omitempty"`
		Time   int     `json:"time,omitempty"`
		Events []Event `json:"messaging,omitempty"`
	} `json:"entry,omitempty"`
}

type Message struct {
	MID        string `json:"mid,omitempty"`
	Text       string `json:"text,omitempty"`
	QuickReply *struct {
		Payload string `json:"payload,omitempty"`
	} `json:"quick_reply,omitempty"`
	Attachments *[]Attachment `json:"attachments,omitempty"`
	Attachment  *Attachment   `json:"attachment,omitempty"`
}

type Attachment struct {
	Type    string  `json:"type,omitempty"`
	Payload Payload `json:"payload,omitempty"`
}

type Postback struct {
	Title    string `json:"title"`
	Payload  string `json:"payload"`
	Referral struct {
		Ref    string `json:"ref"`
		Source string `json:"source"`
		Type   string `json:"type"`
	} `json:"referral"`
}

type Event struct {
	Sender    User     `json:"sender,omitempty"`
	Recipient User     `json:"recipient,omitempty"`
	Timestamp int      `json:"timestamp,omitempty"`
	Message   Message  `json:"message,omitempty"`
	Postback  Postback `json:"postback,omitempty"`
}

func (e Event) IsMessage() bool {
	return e.Message.MID != ""
}

func (e Event) IsPostback() bool {
	return e.Postback.Payload != ""
}

type User struct {
	ID string `json:"id,omitempty"`
}

type Response struct {
	Recipient User    `json:"recipient,omitempty"`
	Message   Message `json:"message,omitempty"`
}

type Payload struct {
	URL string `json:"url,omitempty"`
}

func VerificationHandler(w http.ResponseWriter, r *http.Request) {
	challenge := r.URL.Query().Get("hub.challenge")
	mode := r.URL.Query().Get("hub.mode")
	token := r.URL.Query().Get("hub.verify_token")
	expected := os.Getenv("VERIFY_TOKEN")

	if mode != "" && token == expected {
		log.Printf("verification succeeded")
		w.WriteHeader(200)
		w.Write([]byte(challenge))
	} else {
		log.Printf("verification for token %s failed, expected %s", token, expected)
		w.WriteHeader(404)
		w.Write([]byte("Error, wrong validation token"))
	}
}

func SendMessage(id string, m Message) {
	client := &http.Client{}
	response := Response{
		Recipient: User{
			ID: id,
		},
		Message: m,
	}

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(&response)
	url := fmt.Sprintf(FACEBOOK_API, os.Getenv("PAGE_ACCESS_TOKEN"))
	req, err := http.NewRequest("POST", url, body)
	req.Header.Add("Content-Type", "application/json")
	if err != nil {
		log.Fatal(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
}

func contains(s string, subs ...string) bool {
	str := strings.ToLower(s)
	for _, sub := range subs {
		if strings.Contains(str, sub) {
			return true
		}
	}
	return false
}

var counter = 0

func ProcessMessage(event Event) {
	images := []string{
		"https://pics.me.me/i-could-spat-gopher-a-beer-funny-c3-15199885.png",
		"https://i.pinimg.com/originals/b5/ac/dd/b5acdd83bb12c464bf9d28e107a8fec6.jpg",
		"https://www.memerewards.com/images/2017/12/20/Its__GOPHER_TIME_1513822263b5b3e402ab1a9650.png",
		"https://pics.me.me/vampire-gopher-strikes-again-34149257.png",
	}

	if contains(event.Message.Text, "gopher", "go", "golang") {
		SendMessage(event.Sender.ID, Message{
			Text: "Cool! Here's a gopher for you:",
		})

		SendMessage(event.Sender.ID, Message{
			Attachment: &Attachment{
				Type: "image",
				Payload: Payload{
					URL: images[counter],
				},
			},
		})

		counter = (counter + 1) % len(images)
	} else if contains(event.Message.Text, "python") {
		SendMessage(event.Sender.ID, Message{
			Text: "Go away, you evil person",
		})
	} else if contains(event.Message.Text, "java") {
		SendMessage(event.Sender.ID, Message{
			Text: "I don't mind coding in Java, as long as it's the island in Indonesia",
		})
	}
}

var me string

func CallbackHandler(w http.ResponseWriter, r *http.Request) {
	var callback Callback
	if err := json.NewDecoder(r.Body).Decode(&callback); err != nil {
		log.Println("couldn't decode message: %v", err)
		return
	}

	if callback.Object == "page" {
		for _, entry := range callback.Entry {
			for _, event := range entry.Events {
				// Get the bot's ID from the first message sent. Quick and dirty, but works
				if me == "" {
					me = event.Recipient.ID
				}

				// Process events once me has a value and if the event isn't send by the bot itself
				if event.IsMessage() && me != "" && event.Sender.ID != me {
					log.Printf("Message event of sender ID: %v: %v", event.Sender.ID, event.Message)
					ProcessMessage(event)
				} else if event.IsPostback() && me != "" && event.Sender.ID != me {
					log.Printf("Postback event of sender ID: %v: %v", event.Sender.ID, event.Postback)
				}
			}
		}
		w.WriteHeader(200)
		w.Write([]byte("EVENT_RECEIVED"))
	} else {
		w.WriteHeader(404)
		w.Write([]byte("CALLBACK_ERROR"))
	}
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/webhook", VerificationHandler).Methods("GET")
	r.HandleFunc("/webhook", CallbackHandler).Methods("POST")
	if err := http.ListenAndServe("0.0.0.0:8080", r); err != nil {
		log.Fatal(err)
	}
}