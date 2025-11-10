package main

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	fmt.Println("Starting Peril client...")
	const rabbitConnString = "amqp://guest:guest@localhost:5672/"

	conn, err := amqp.Dial(rabbitConnString)
	if err != nil {
		log.Fatalf("could not connect to RabbitMQ: %v", err)
	}
	defer conn.Close()
	fmt.Println("Peril game client connected to RabbitMQ!")

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("could not open channel: %v", err)
	}

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		log.Fatalf("could not get username: %v", err)
	}

	// // Previous declare and bind
	// _, queue, err := pubsub.DeclareAndBind(
	// 	conn,
	// 	routing.ExchangePerilDirect,
	// 	fmt.Sprintf("%s.%s", routing.PauseKey, username),
	// 	routing.PauseKey,
	// 	pubsub.SimpleQueueTransient,
	// )
	// if err != nil {
	// 	log.Fatalf("could not subscribe to %s: %v", routing.PauseKey, err)
	// }
	// fmt.Printf("Queue %v declared and bound!\n", queue.Name)

	gs := gamelogic.NewGameState(username)

	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilDirect,
		routing.PauseKey+"."+gs.GetUsername(),
		routing.PauseKey,
		pubsub.SimpleQueueTransient,
		handlerPause(gs),
	)
	if err != nil {
		log.Fatalf("could not subscribe to %s: %v", routing.PauseKey, err)
	}

	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilTopic,
		routing.ArmyMovesPrefix+"."+gs.GetUsername(),
		routing.ArmyMovesPrefix+".*",
		pubsub.SimpleQueueTransient,
		handlerMove(gs, ch),
	)
	if err != nil {
		log.Fatalf("could not subscribe to %s: %v", routing.ArmyMovesPrefix+".*", err)
	}

	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilTopic,
		routing.WarRecognitionsPrefix,
		routing.WarRecognitionsPrefix+".*",
		pubsub.SimpleQueueDurable,
		handlerWar(gs, ch),
	)
	if err != nil {
		log.Fatalf("could not subscribe to %s: %v", routing.WarRecognitionsPrefix+".*", err)
	}

	for {
		userInput := gamelogic.GetInput()

		if len(userInput) == 0 {
			continue
		}

		switch userInput[0] {
		case "spawn":
			err = gs.CommandSpawn(userInput)
			if err != nil {
				fmt.Println(err.Error())
			}
		case "move":
			move, err := gs.CommandMove(userInput)
			if err != nil {
				fmt.Println(err.Error())
			}

			err = pubsub.PublishJSON(
				ch,
				routing.ExchangePerilTopic,
				routing.ArmyMovesPrefix+"."+gs.GetUsername(),
				move,
			)
			if err != nil {
				fmt.Println(err.Error())
			} else {
				fmt.Println("Move published successfully!")
			}
		case "status":
			gs.CommandStatus()
		case "help":
			gamelogic.PrintClientHelp()
		case "spam":
			if len(userInput) != 2 {
				fmt.Println("usage: spam <number>")
				continue
			}

			msgNumber, err := strconv.Atoi(userInput[1])
			if err != nil {
				fmt.Println("usage: spam <number>")
				continue	
			}

			for range(msgNumber) {
				message := gamelogic.GetMaliciousLog()
				pubsub.PublishGob(
					ch,
					routing.ExchangePerilTopic,
					routing.GameLogSlug + "." + gs.GetUsername(),
					routing.GameLog{
						CurrentTime: time.Now(),
						Message: message,
						Username: gs.GetUsername(),
					},
				)
			}
			
		case "quit":
			gamelogic.PrintQuit()
			return
		default:
			fmt.Println("Unknown command!")

		}
	}

	// wait for ctrl+c
	// signalChan := make(chan os.Signal, 1)
	// signal.Notify(signalChan, os.Interrupt)
	// <-signalChan
	// fmt.Println("RabbitMQ connection closed.")
}
