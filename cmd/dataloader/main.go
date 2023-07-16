package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"runtime"
	"sync"

	"github.com/caarlos0/env/v6"
	"github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog/log"

	"github.com/lesnoi-kot/versions-backend/dataloader"
	"github.com/lesnoi-kot/versions-backend/mongostore"
	"github.com/lesnoi-kot/versions-backend/mq"
)

var store *mongostore.Store

type AppConfig struct {
	MongoURI  string `env:"MONGO_URI,notEmpty"`
	RabbitURI string `env:"RABBIT_URI,notEmpty"`
}

func main() {
	config := new(AppConfig)
	if err := env.Parse(config); err != nil {
		log.Fatal().Err(err).Msg("Config parsing error")
	}

	globalCtx, cancel := context.WithCancel(context.Background())
	go handleSignals(cancel)

	var err error

	if store, err = mongostore.ConnectStore(globalCtx, config.MongoURI); err != nil {
		log.Fatal().Err(err).Msg("Mongo connection error")
	}

	log.Info().Msg("Mongo connection established")
	defer store.Disconnect(globalCtx)

	amqp, err := mq.NewAMQPConnection(config.RabbitURI)
	if err != nil {
		log.Fatal().Err(err).Msg("RabbitMQ connection error")
	}

	log.Info().Msg("RabbitMQ connection established")
	defer amqp.Close()

	var wg sync.WaitGroup
	wg.Add(runtime.NumCPU())

	for i := 0; i < runtime.NumCPU(); i++ {
		go func() {
			handleSourceRequests(globalCtx, amqp, store)
			wg.Done()
		}()
	}

	if err != nil {
		log.Printf("HandleSourceRequests error: %s", err)
	}

	wg.Wait()
}

func handleSignals(cancel context.CancelFunc) {
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)

	<-quit
	cancel()
}

func handleSourceRequests(ctx context.Context, conn *mq.AMQPConnection, store *mongostore.Store) error {
	ch, err := conn.Channel()
	if err != nil {
		return err
	}

	defer ch.Close()

	msgs, err := mq.ConsumeRepoRequestsQueue(ch)
	if err != nil {
		return err
	}

	log.Info().Msg("Queue channel initialized. Ready to handle messages")

	for {
		select {
		case <-ctx.Done():
			return nil
		case msg := <-msgs:
			handleMessage(ctx, &msg)
		}
	}
}

func handleMessage(ctx context.Context, msg *amqp091.Delivery) {
	if deathInfos, ok := msg.Headers["x-death"].([]interface{}); ok && len(deathInfos) > 0 {
		if deathInfo, ok := deathInfos[0].(amqp091.Table); ok {
			if deathCount, ok := deathInfo["count"].(int64); ok && deathCount > 10 {
				log.Info().Msgf("Discarding message with x-death[0].count > 10")
				msg.Ack(false)
				return
			}
		}
	}

	body := new(mq.GithubRepoRequestMessage)

	if err := json.Unmarshal(msg.Body, body); err != nil {
		log.Error().Err(err).Msgf(`Invalid RabbitMQ message body: "%s"`, string(msg.Body))
		msg.Ack(false)
		return
	}

	loader := dataloader.NewGithubReleaseLoader(dataloader.GithubReleaseLoaderConfig{
		MongoStore: store,
		Message:    body,
	})

	if err := loader.Dispatch(ctx); err != nil {
		log.Error().Err(err).Msg(`Github release loader failed`)

		if err == dataloader.ErrRateLimit {
			log.Info().Msgf(`Rate limit error encountered, retry current message later`)
			msg.Reject(false) // Send to retry queue
		} else {
			msg.Ack(false)
		}
	} else {
		log.Info().Msg("Message successfully dispatched")
		msg.Ack(false)
	}
}
