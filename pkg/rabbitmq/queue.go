package rabbitmq

import (
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/huo-ju/dfserver/pkg/data"
	"github.com/streadway/amqp"
)

// Queue wrapping the amqp Channel operation and manage the connection.
type AmqpQueue struct {
	AmqpChannel    *Channel
	Conn           *Connection
	BaseRetryDelay uint
	MaxRetries     uint
}

// Close the channel and connection
func (q *AmqpQueue) Close() {
	q.AmqpChannel.Close()
	q.Conn.Close()
}

// Consume queue with route_key
func (q *AmqpQueue) Consume(name string, key string, qoscount int) (<-chan amqp.Delivery, error) {
	q.AmqpChannel.Qos(qoscount, 0, false)
	return q.AmqpChannel.Consume(fmt.Sprintf("%s.%s", name, key), "", false, false, false, false, nil)
}

// Publish send task to the queue with route_key and priority
func (q *AmqpQueue) PublishExchangePriority(exchange string, key string, body []byte, priority uint8) error {
	return q.AmqpChannel.Publish(exchange, key, false, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		Priority:     priority,
		ContentType:  "text/plain",
		Body:         body,
	})
}

func (q *AmqpQueue) PublishPriority(key string, body []byte, priority uint8) error {
	return q.AmqpChannel.Publish("Name", key, false, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		Priority:     priority,
		ContentType:  "text/plain",
		Body:         body,
	})
}

func (q *AmqpQueue) Declare(queues map[string]data.QueueItem) error {
	var err error
	for _, v := range queues {
		err = q.AmqpChannel.ExchangeDeclare(v.Name, "direct", true, false, false, false, nil)
		if err != nil {
			return err
		}
		for _, key := range v.Bindkeys {
			key = strings.TrimSpace(key)
			_, err = q.AmqpChannel.QueueDeclare(fmt.Sprintf("%s.%s", v.Name, key), true, false, false, false, nil)
			if err != nil {
				return err
			}
			err = q.AmqpChannel.QueueBind(fmt.Sprintf("%s.%s", v.Name, key), key, v.Name, false, nil)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Init the Queue and return a Queue instance
func Init(connectstr string, baseRetryDelay uint, maxRetries uint, config *Config, tlsconfig *tls.Config) (*AmqpQueue, error) {
	conn, err := Dial(connectstr, config, tlsconfig)
	if err != nil {
		return nil, err
	}

	amqpChannel, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	queue := &AmqpQueue{AmqpChannel: amqpChannel, Conn: conn, BaseRetryDelay: baseRetryDelay, MaxRetries: maxRetries}
	return queue, err
}
