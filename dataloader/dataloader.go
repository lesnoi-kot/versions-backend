package dataloader

import (
	"context"
	"encoding/json"

	"github.com/lesnoi-kot/versions-backend/mongostore"
	"github.com/lesnoi-kot/versions-backend/mq"
	"github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog/log"
)

type Dataloader struct {
	MQ    *mq.AMQPConnection
	Store *mongostore.Store
}

func (dataloader Dataloader) Serve(ctx context.Context) error {
	ch, err := dataloader.MQ.Channel()
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
			dataloader.handleMessage(ctx, &msg)
		}
	}
}

func (dataloader Dataloader) handleMessage(ctx context.Context, msg *amqp091.Delivery) {
	if deathCount := mq.GetDeliveryDeathCount(msg); deathCount > 10 {
		log.Info().Msgf("Discarding message with x-death count > 10")
		msg.Ack(false)
		return
	}

	body := new(mq.GithubRepoRequestMessage)

	if err := json.Unmarshal(msg.Body, body); err != nil {
		log.Error().Err(err).Msgf(`Invalid RabbitMQ message body: "%s"`, string(msg.Body))
		msg.Ack(false)
		return
	}

	loader := NewGithubReleaseLoader(GithubReleaseLoaderConfig{
		MongoStore: dataloader.Store,
		Message:    body,
	})

	err := loader.Dispatch(ctx)

	if err == nil {
		log.Info().Msg("Message successfully dispatched")
		msg.Ack(false)
	} else if err == context.Canceled {
		log.Info().Msgf("Message handling process is canceled")
		msg.Nack(false, true)
	} else if err == ErrRateLimit {
		log.Info().Msgf(`Rate limit error encountered, retry current message later`)
		msg.Reject(false) // Send to retry queue
	} else {
		log.Error().Err(err).Msg(`Github release loader failed`)
		msg.Ack(false)
	}
}
