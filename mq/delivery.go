package mq

import "github.com/rabbitmq/amqp091-go"

func GetDeliveryDeathCount(msg *amqp091.Delivery) int64 {
	if deathInfos, ok := msg.Headers["x-death"].([]interface{}); ok && len(deathInfos) > 0 {
		if deathInfo, ok := deathInfos[0].(amqp091.Table); ok {
			if deathCount, ok := deathInfo["count"].(int64); ok {
				return deathCount
			}
		}
	}

	return 0
}
