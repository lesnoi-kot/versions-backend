package main

import (
	"context"
	"log"
	"net/http"

	"github.com/caarlos0/env/v6"
	"github.com/lesnoi-kot/versions-backend/api"
	"github.com/lesnoi-kot/versions-backend/common"
	"github.com/lesnoi-kot/versions-backend/mongostore"
	"github.com/lesnoi-kot/versions-backend/mq"
)

type AppConfig struct {
	Debug        bool     `env:"DEBUG" envDefault:"true"`
	MongoURI     string   `env:"MONGO_URI,notEmpty"`
	RabbitURI    string   `env:"RABBIT_URI,notEmpty"`
	AllowOrigins []string `env:"ALLOW_ORIGINS" envDefault:"*"`
}

func main() {
	config := new(AppConfig)
	if err := env.Parse(config); err != nil {
		log.Fatalf("Config parsing error: %s", err)
	}

	globalCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go common.HandleInterruptSignal(cancel)

	store, err := mongostore.ConnectStore(globalCtx, config.MongoURI)
	if err != nil {
		log.Fatalf("Mongo connection error: %s", err)
	}

	log.Println("Mongo connection established")
	defer store.Disconnect(globalCtx)

	amqp, err := mq.NewAMQPConnection(config.RabbitURI)
	if err != nil {
		log.Fatalf("RabbitMQ connection error: %s", err)
	}

	log.Println("RabbitMQ connection established")
	defer amqp.Close()

	apiService := api.NewAPI(api.APIConfig{
		Store:        store,
		MQ:           amqp,
		AllowOrigins: config.AllowOrigins,
		Debug:        config.Debug,
	})

	if err := apiService.Start(":4000"); err != nil {
		log.Println("API service is stopped")

		if err != http.ErrServerClosed {
			log.Printf("Server stopped with an error: %s", err)
		}
	}
}
