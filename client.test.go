package main

import (
	"log"
	"os"
	"os/signal"
	"testing"
)

func TestQuery(t *testing.T) {
	// Configure the client
	token := os.Getenv("TOKEN")
	client := Client{
		token:                token,
		subscriptionEndpoint: `wss://v1.igloo.ooo/subscriptions`,
		queryEndpoint:        "https://v1.igloo.ooo/graphql",
	}

	// Execute a query
	resp, queryErr := client.Query(` {
		floatVariable(id: "c3fd95b8-5333-4d81-9fbf-0323398ec66b") {
			id
			value
		}
	}`)
	if queryErr != nil {
		log.Fatal(queryErr)
	}
	log.Println("Initial Value: ", resp["floatVariable"].(map[string]interface{})["value"])
}

func BenchmarkSubscription(t *testing.B) {
	// Configure the client
	token := os.Getenv("TOKEN")
	client := Client{
		token:                token,
		subscriptionEndpoint: `wss://v1.igloo.ooo/subscriptions`,
		queryEndpoint:        "https://v1.igloo.ooo/graphql",
	}

	// Open the subscription
	subscriptionQuery := `subscription{variableUpdated(id: "c3fd95b8-5333-4d81-9fbf-0323398ec66b"){id ...on FloatVariable{value}}}`
	messages, Stop, err := client.Subscribe(subscriptionQuery)
	if err != nil {
		log.Fatal(err)
	}

	// Listen for CTRL-C to Stop the subscription
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	go func() {
		<-interrupt
		Stop()
	}()

	// Iterate messages and print them to console
	i := 0
	for message := range messages {
		i++
		if message.Error != nil {
			log.Fatal(message.Error)
		}

		data := (*message.Payload)["variableUpdated"].(map[string]interface{})
		log.Println("Value: ", data["value"])

		if i > 2 {
			break
		}
	}
}
