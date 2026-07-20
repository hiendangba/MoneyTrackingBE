package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"auth-service/internal/config"

	amqp "github.com/rabbitmq/amqp091-go"
)

type OTPPublisher interface {
	PublishRegisterOTP(ctx context.Context, email, fullName, otp string, expiresIn time.Duration) error
	PublishResetOTP(ctx context.Context, email, otp string, expiresIn time.Duration) error
	Close() error
}

type RabbitMQPublisher struct {
	conn        *amqp.Connection
	channel     *amqp.Channel
	exchange    string
	registerKey string
	resetKey    string
}

type OTPMessage struct {
	Type             string `json:"type"`
	Email            string `json:"email"`
	FullName         string `json:"full_name,omitempty"`
	OTPCode          string `json:"otp_code"`
	ExpiresInSeconds int64  `json:"expires_in_seconds"`
}

func NewRabbitMQPublisher(cfg config.Config) (*RabbitMQPublisher, error) {
	conn, err := amqp.Dial(cfg.RabbitMQ.URL)
	if err != nil {
		return nil, fmt.Errorf("dial rabbitmq: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("open rabbitmq channel: %w", err)
	}

	if err := channel.ExchangeDeclare(
		cfg.RabbitMQ.Exchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		_ = channel.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("declare exchange: %w", err)
	}

	return &RabbitMQPublisher{
		conn:        conn,
		channel:     channel,
		exchange:    cfg.RabbitMQ.Exchange,
		registerKey: cfg.RabbitMQ.RegisterKey,
		resetKey:    cfg.RabbitMQ.ResetKey,
	}, nil
}

func (p *RabbitMQPublisher) PublishRegisterOTP(ctx context.Context, email, fullName, otp string, expiresIn time.Duration) error {
	return p.publish(ctx, p.registerKey, OTPMessage{
		Type:             "register_otp",
		Email:            email,
		FullName:         fullName,
		OTPCode:          otp,
		ExpiresInSeconds: int64(expiresIn.Seconds()),
	})
}

func (p *RabbitMQPublisher) PublishResetOTP(ctx context.Context, email, otp string, expiresIn time.Duration) error {
	return p.publish(ctx, p.resetKey, OTPMessage{
		Type:             "reset_password_otp",
		Email:            email,
		OTPCode:          otp,
		ExpiresInSeconds: int64(expiresIn.Seconds()),
	})
}

func (p *RabbitMQPublisher) publish(ctx context.Context, routingKey string, message OTPMessage) error {
	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal otp message: %w", err)
	}

	if err := p.channel.PublishWithContext(
		ctx,
		p.exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
			Timestamp:   time.Now(),
		},
	); err != nil {
		return fmt.Errorf("publish otp message: %w", err)
	}

	return nil
}

func (p *RabbitMQPublisher) Close() error {
	if p.channel != nil {
		if err := p.channel.Close(); err != nil {
			_ = p.conn.Close()
			return err
		}
	}
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}
