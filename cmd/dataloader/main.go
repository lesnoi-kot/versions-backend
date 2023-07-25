package main

import (
	"context"
	"runtime"
	"sync"

	"github.com/caarlos0/env/v6"
	"github.com/rs/zerolog/log"

	"github.com/lesnoi-kot/versions-backend/common"
	"github.com/lesnoi-kot/versions-backend/dataloader"
	"github.com/lesnoi-kot/versions-backend/mongostore"
	"github.com/lesnoi-kot/versions-backend/mq"
)

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
	defer cancel()
	go common.HandleInterruptSignal(cancel)

	store, err := mongostore.ConnectStore(globalCtx, config.MongoURI)
	if err != nil {
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

	dataloader := &dataloader.Dataloader{
		MQ:    amqp,
		Store: store,
	}

	var wg sync.WaitGroup
	wg.Add(runtime.NumCPU())

	for i := 0; i < runtime.NumCPU(); i++ {
		go func() {
			if err := dataloader.Serve(globalCtx); err != nil {
				log.Error().Err(err).Msgf("dataloader.Serve error: %s", err)
			}

			wg.Done()
		}()
	}

	wg.Wait()
}
