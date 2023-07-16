package mq

import (
	"context"
	"encoding/json"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	retriesExchangeName = "dlx-source-requests"
	retryDelay          = time.Minute
)

type AMQPConnection struct {
	*amqp.Connection
}

func NewAMQPConnection(amqpURI string) (*AMQPConnection, error) {
	conn, err := amqp.Dial(amqpURI)
	if err != nil {
		return nil, err
	}

	return &AMQPConnection{conn}, nil
}

// Message format of a repo request.
type GithubRepoRequestMessage struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

func DeclareRepoRequestsQueue(ch *amqp.Channel) (amqp.Queue, error) {
	// Dead letter exchange for rejected requests.
	err := ch.ExchangeDeclare(
		retriesExchangeName,
		"fanout",
		true,  // durable
		false, // autoDelete
		false, // internal
		false, // noWait
		nil,
	)
	if err != nil {
		return amqp.Queue{}, err
	}

	retryQueue, err := ch.QueueDeclare(
		"dlq-source-requests",
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		amqp.Table{
			"x-dead-letter-exchange": "", // Return message to the default exchange
			"x-message-ttl":          retryDelay.Milliseconds(),
		},
	)
	if err != nil {
		return amqp.Queue{}, err
	}

	if err = ch.QueueBind(retryQueue.Name, "", retriesExchangeName, false, nil); err != nil {
		return amqp.Queue{}, err
	}

	return ch.QueueDeclare(
		"source-requests",
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		amqp.Table{
			"x-dead-letter-exchange": retriesExchangeName,
		},
	)
}

func ConsumeRepoRequestsQueue(ch *amqp.Channel) (<-chan amqp.Delivery, error) {
	q, err := DeclareRepoRequestsQueue(ch)
	if err != nil {
		return nil, err
	}

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)

	return msgs, err
}

func (conn *AMQPConnection) PushRepoRequest(ctx context.Context, req *GithubRepoRequestMessage) error {
	ch, err := conn.Channel()
	if err != nil {
		return err
	}

	q, err := DeclareRepoRequestsQueue(ch)
	if err != nil {
		return err
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	return ch.PublishWithContext(
		ctx,
		"",     // exchange
		q.Name, // key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			ContentType: "text/json",
			Body:        body,
		},
	)
}
