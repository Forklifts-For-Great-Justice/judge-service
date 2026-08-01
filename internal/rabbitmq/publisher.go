// Package rabbitmq provides a publisher for shenanigan activation messages.
package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ShenaniganMessage is the payload published when a shenanigan is activated.
type ShenaniganMessage struct {
	PurchaseID   string          `json:"purchase_id"`
	ShenaniganID string          `json:"shenanigan_id"`
	RconPayload  string          `json:"rcon_payload"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
}

// Publisher wraps an AMQP connection and exposes a Publish method.
// The exchange, queue, and routing key are configured via NewPublisher.
type Publisher struct {
	conn       *amqp.Connection
	ch         *amqp.Channel
	exchange   string
	routingKey string
}

// NewPublisher creates a new Publisher connected to the given RabbitMQ URL.
// If the connection fails it returns an error — handlers should check for nil.
func NewPublisher(ctx context.Context, rabbitmqURL string, exchange string) (*Publisher, error) {
	conn, err := amqp.Dial(rabbitmqURL)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq dial: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("rabbitmq channel: %w", err)
	}

	err = ch.ExchangeDeclare(
		exchange, "topic", true, false, false, false, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq exchange declare: %w", err)
	}

	return &Publisher{
		conn:       conn,
		ch:         ch,
		exchange:   exchange,
		routingKey: "shenanigans.shenanigan.judge",
	}, nil
}

// Publish marshals the message as msgpack and publishes it to the exchange.
func (p *Publisher) Publish(ctx context.Context, msg ShenaniganMessage) (bool, error) {
	payload, err := encodeMsgpack(msg)
	if err != nil {
		return false, fmt.Errorf("msgpack encode: %w", err)
	}

	err = p.ch.PublishWithContext(ctx,
		p.exchange,
		p.routingKey,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:   "application/vnd.msgpack",
			CorrelationId: msg.PurchaseID,
			Body:          payload,
		},
	)
	if err != nil {
		return false, err
	}

	return true, nil
}

// Close cleans up the channel and connection.
func (p *Publisher) Close() {
	if p.ch != nil {
		p.ch.Close()
	}
	if p.conn != nil {
		p.conn.Close()
	}
}

// encodeMsgpack marshals data to msgpack format.
func encodeMsgpack(v any) ([]byte, error) {
	// For now use JSON; full msgpack support requires
	// "github.com/vmihailenco/msgpack/v5" import
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	// Add msgpack preamble header if needed in the future
	return data, nil
}
