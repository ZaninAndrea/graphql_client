package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type SubscriptionMessage struct {
	Payload *map[string]interface{}
	Error   error
}

func ReadMessagesToChannel(c *websocket.Conn, messages chan []byte, errors chan SubscriptionMessage) {
	for {
		_, rawMessage, err := c.ReadMessage()

		if err != nil {
			// Detect error raised on closing the connection intentionally
			if err.Error() == "websocket: close 1000 (normal)" {
				close(messages)
				return
			}

			errors <- SubscriptionMessage{
				Payload: nil,
				Error:   err,
			}
			return
		}

		messages <- rawMessage
	}
}

// TODO use a single websocket to listen for multiple subscriptions
func (client *Client) Subscribe(query string) (chan SubscriptionMessage, func(), error) {
	emptyFunc := func() {}
	// Opening connection
	u, err := url.Parse(client.subscriptionEndpoint)
	if err != nil {
		return nil, emptyFunc, err
	}

	dialer := websocket.Dialer{Subprotocols: []string{"graphql-ws"}}
	c, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		return nil, emptyFunc, err
	}

	// Sending handshake message
	handshakeMessage := fmt.Sprintf(`{"type":"connection_init","payload":{"Authorization":"Bearer %s"}}`, client.token)
	err = c.WriteMessage(websocket.TextMessage, []byte(handshakeMessage))
	if err != nil {
		return nil, emptyFunc, err
	}

	// Receiving connection_ack
	_, rawMessage, err := c.ReadMessage()
	if err != nil {
		return nil, emptyFunc, err
	}
	var message struct {
		Type string
	}
	err = json.Unmarshal([]byte(rawMessage), &message)
	if err != nil {
		return nil, emptyFunc, err
	}
	if message.Type != "connection_ack" {
		return nil, emptyFunc, err
	}

	// Sending subscription message
	query = strings.ReplaceAll(query, `"`, `\"`)
	query = strings.ReplaceAll(query, "\n", `\n`)
	subscriptionQueryMessage := fmt.Sprintf(`{"id":"1","type":"start","payload":{"query":"%s","variables":null}}`, query)
	err = c.WriteMessage(websocket.TextMessage, []byte(subscriptionQueryMessage))
	if err != nil {
		return nil, emptyFunc, err
	}

	// Listening for responses
	messages := make(chan SubscriptionMessage)
	interrupt := make(chan int)

	go func() {
		defer close(messages)
		defer c.Close()

		socketMessages := make(chan []byte)
		go ReadMessagesToChannel(c, socketMessages, messages)

		for {
			select {
			case <-interrupt:
				// Cleanly close the connection by sending a close message and then
				// waiting (with timeout) for the server to close the connection.
				err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				if err != nil {
					messages <- SubscriptionMessage{
						Payload: nil,
						Error:   err,
					}
					return
				}

				select {
				case <-socketMessages:
				case <-time.After(2 * time.Second):
				}

				return
			case rawMessage := <-socketMessages:
				// Parse JSON to struct
				var message struct {
					Type    string `json:"type"`
					Payload *struct {
						Data   map[string]interface{} `json:"data"`
						Errors *[]struct {
							Message string
						} `json:"errors"`
					} `json:"payload"`
				}
				err = json.Unmarshal([]byte(rawMessage), &message)
				if err != nil {
					messages <- SubscriptionMessage{
						Payload: nil,
						Error:   err,
					}
					return
				}

				// Read message content
				if message.Type == "ka" {
					continue
				} else if message.Type == "data" {
					if message.Payload.Errors != nil {
						errorString, err := json.Marshal(message.Payload.Errors)
						if err != nil {
							messages <- SubscriptionMessage{
								Payload: nil,
								Error:   err,
							}
						} else {
							messages <- SubscriptionMessage{
								Payload: nil,
								Error:   fmt.Errorf(string(errorString)),
							}
						}

						return
					}

					messages <- SubscriptionMessage{
						Payload: &message.Payload.Data,
						Error:   nil,
					}
				} else {
					messages <- SubscriptionMessage{
						Payload: nil,
						Error:   fmt.Errorf("unknown message type %s", message.Type),
					}
					return
				}
			}

		}
	}()

	Stop := func() {
		interrupt <- 0
	}

	return messages, Stop, nil
}
