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
	MID         string        `json:"mid,omitempty"`
	Text        string        `json:"text,omitempty"`
	Attachments *[]Attachment `json:"attachments,omitempty"`
	Attachment  *Attachment   `json:"attachment,omitempty"`
}

type Payload map[string]interface{}

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
	Sender    User      `json:"sender,omitempty"`
	Recipient User      `json:"recipient,omitempty"`
	Timestamp int       `json:"timestamp,omitempty"`
	Message   *Message  `json:"message,omitempty"`
	Postback  *Postback `json:"postback,omitempty"`
}

func (e Event) IsMessage() bool {
	return e.Message != nil
}

func (e Event) IsPostback() bool {
	return e.Postback != nil
}

type User struct {
	ID string `json:"id,omitempty"`
}

type Response struct {
	Recipient User    `json:"recipient,omitempty"`
	Message   Message `json:"message,omitempty"`
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

func SendMessage(id string, m Message) (*MessageResponse, error) {
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

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	resp := MessageResponse{}
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
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

type MessageResponse struct {
	RecipientID  string `json:"recipient_id"`
	MessageID    string `json:"message_id"`
	AttachmentID string `json:"attachment_id,omitempty"`
}

type M map[string]interface{}

var (
	counter  int
	owner    string
	imageIDs []string
	images   = []string{
		"https://pics.me.me/i-could-spat-gopher-a-beer-funny-c3-15199885.png",
		"https://i.pinimg.com/originals/b5/ac/dd/b5acdd83bb12c464bf9d28e107a8fec6.jpg",
		"https://www.memerewards.com/images/2017/12/20/Its__GOPHER_TIME_1513822263b5b3e402ab1a9650.png",
		"https://pics.me.me/vampire-gopher-strikes-again-34149257.png",
	}
	me string
)

func sendGopher(id string) {
	SendMessage(id, Message{
		Text: "Cool! Here's a gopher for you:",
	})

	SendMessage(id, Message{
		Attachment: &Attachment{
			Type: "template",
			Payload: Payload{
				"template_type": "media",
				"elements": []M{
					M{
						"media_type":    "image",
						"attachment_id": imageIDs[counter],
						"buttons": []M{
							M{
								"type":  "web_url",
								"title": "Website",
								"url":   "http://golang.org",
							},
							M{
								"type":    "postback",
								"title":   "I can go-pher more",
								"payload": "_GOPHER",
							},
						},
					},
				},
			},
		},
	})
	counter = (counter + 1) % len(images)
}

func ProcessMessage(event Event) {
	if event.Message.Text == "_INIT" {
		if owner != "" {
			return
		}

		owner = event.Sender.ID
		for _, image := range images {
			resp, err := SendMessage(owner, Message{
				Attachment: &Attachment{
					Type: "image",
					Payload: Payload{
						"url":         image,
						"is_reusable": true,
					},
				},
			})

			if err != nil {
				log.Printf("init error: %v", err)
				continue
			}

			imageIDs = append(imageIDs, resp.AttachmentID)
		}
		SendMessage(event.Sender.ID, Message{
			Text: fmt.Sprintf("The bot is initialized, attachment IDs are: %v", imageIDs),
		})

	} else if contains(event.Message.Text, "gopher", "go", "golang") && owner != "" {
		sendGopher(event.Sender.ID)
	} else if contains(event.Message.Text, "python") && owner != "" {
		SendMessage(event.Sender.ID, Message{
			Text: "Go away, you evil person",
		})
	} else if contains(event.Message.Text, "java") && owner != "" {
		SendMessage(event.Sender.ID, Message{
			Text: "I don't mind coding in Java, as long as it's the island in Indonesia",
		})
	} else {
		SendMessage(event.Sender.ID, Message{
			Text: "This bot isn't initialized",
		})
	}
}

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
					if event.Postback.Payload == "_GOPHER" {
						sendGopher(event.Sender.ID)
					}
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
