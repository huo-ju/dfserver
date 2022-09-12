package rabbitmq

import (
	"crypto/tls"
	"log"
	"sync/atomic"
	"time"

	"github.com/streadway/amqp"
)

const delay = 3 // reconnect after delay seconds

// Connection amqp.Connection wrapper
type Connection struct {
	*amqp.Connection
	*Config
}

type Config struct {
	Qos int
}

// Channel wrap amqp.Connection.Channel, get a auto reconnect channel
func (c *Connection) Channel() (*Channel, error) {
	ch, err := c.Connection.Channel()

	if err != nil {
		return nil, err
	}

	if c.Config.Qos > 0 {
		ch.Qos(c.Config.Qos, 0, false)
	}

	channel := &Channel{
		Channel: ch,
	}

	go func() {
		for {
			reason, ok := <-channel.Channel.NotifyClose(make(chan *amqp.Error))
			// exit this goroutine if closed by developer
			if !ok || channel.IsClosed() {
				log.Println("channel closed")
				channel.Close() // close again, ensure closed flag set when connection closed
				break
			}
			log.Println("channel closed, reason: %v", reason)

			// reconnect if not closed by developer
			for {
				// wait 1s for connection reconnect
				time.Sleep(delay * time.Second)

				ch, err := c.Connection.Channel()
				if err == nil {
					if c.Config.Qos > 0 {
						ch.Qos(c.Config.Qos, 0, false)
					}
					log.Println("channel recreate success")
					channel.Channel = ch
					break
				}

				log.Println("channel recreate failed, err: %v", err)
			}
		}

	}()

	return channel, nil
}

// Dial wrap amqp.Dial / amqp.DialTLS, dial and get a reconnect connection
func Dial(url string, conf *Config, tlsconf *tls.Config) (*Connection, error) {
	var conn *amqp.Connection
	var err error
	if tlsconf == nil {
		conn, err = amqp.Dial(url)
	} else {
		conn, err = amqp.DialTLS(url, tlsconf)
	}
	if err != nil {
		return nil, err
	}

	connection := &Connection{
		Connection: conn,
		Config:     conf,
	}

	go func() {
		for {
			reason, ok := <-connection.Connection.NotifyClose(make(chan *amqp.Error))
			// exit this goroutine if closed by developer
			if !ok {
				log.Println("connection closed")
				break
			}
			log.Println("connection closed, reason: %v", reason)

			// reconnect if not closed by developer
			for {
				// wait 1s for reconnect
				time.Sleep(delay * time.Second)

				var conn *amqp.Connection
				var err error
				if tlsconf == nil {
					conn, err = amqp.Dial(url)
				} else {
					conn, err = amqp.DialTLS(url, tlsconf)
				}
				if err == nil {
					connection.Connection = conn
					log.Println("reconnect success")
					break
				}

				log.Println("reconnect failed, err: %v", err)
			}
		}
	}()

	return connection, nil
}

// Channel amqp.Channel wapper
type Channel struct {
	*amqp.Channel
	closed int32
}

// IsClosed indicate closed by developer
func (ch *Channel) IsClosed() bool {
	return (atomic.LoadInt32(&ch.closed) == 1)
}

// Close ensure closed flag set
func (ch *Channel) Close() error {
	if ch.IsClosed() {
		return amqp.ErrClosed
	}

	atomic.StoreInt32(&ch.closed, 1)

	return ch.Channel.Close()
}

// Consume warp amqp.Channel.Consume, the returned delivery will end only when channel closed by developer
func (ch *Channel) Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error) {
	deliveries := make(chan amqp.Delivery)

	go func() {
		for {
			d, err := ch.Channel.Consume(queue, consumer, autoAck, exclusive, noLocal, noWait, args)
			if err != nil {
				log.Println("consume failed, err: %v", err)
				time.Sleep(delay * time.Second)
				continue
			}

			for msg := range d {
				deliveries <- msg
			}

			// sleep before IsClose call. closed flag may not set before sleep.
			time.Sleep(delay * time.Second)

			if ch.IsClosed() {
				break
			}
		}
	}()

	return deliveries, nil
}
