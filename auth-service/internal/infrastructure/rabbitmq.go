package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/internal/config"

	amqp "github.com/rabbitmq/amqp091-go"
)

const rabbitPublishConfirmTimeout = 5 * time.Second

type OTPPublisher interface {
	PublishRegisterOTP(ctx context.Context, email, fullName, otp string, expiresIn time.Duration) error
	PublishResetOTP(ctx context.Context, email, otp string, expiresIn time.Duration) error
	Close() error
}

type RabbitMQPublisher struct {
	mu          sync.Mutex
	conn        *amqp.Connection
	channel     *amqp.Channel
	url         string
	exchange    string
	emailQueue  string
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
	publisher := &RabbitMQPublisher{
		url:         cfg.RabbitMQ.URL,
		exchange:    cfg.RabbitMQ.Exchange,
		emailQueue:  cfg.RabbitMQ.EmailQueue,
		registerKey: cfg.RabbitMQ.RegisterKey,
		resetKey:    cfg.RabbitMQ.ResetKey,
	}
	if err := publisher.connectLocked(); err != nil {
		return nil, err
	}
	return publisher, nil
}

func (p *RabbitMQPublisher) connectLocked() error {
	_ = p.closeLocked() // A stale socket close error must not block a fresh connection.

	conn, err := amqp.Dial(p.url)
	if err != nil {
		return fmt.Errorf("dial rabbitmq: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("open rabbitmq channel: %w", err)
	}

	if err := channel.ExchangeDeclare(
		p.exchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		_ = channel.Close()
		_ = conn.Close()
		return fmt.Errorf("declare exchange: %w", err)
	}

	queue, err := channel.QueueDeclare(
		p.emailQueue,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		_ = channel.Close()
		_ = conn.Close()
		return fmt.Errorf("declare email queue: %w", err)
	}

	for _, key := range []string{p.registerKey, p.resetKey} {
		if err := channel.QueueBind(queue.Name, key, p.exchange, false, nil); err != nil {
			_ = channel.Close()
			_ = conn.Close()
			return fmt.Errorf("bind email queue with routing key %s: %w", key, err)
		}
	}

	if err := channel.Confirm(false); err != nil {
		_ = channel.Close()
		_ = conn.Close()
		return fmt.Errorf("enable rabbitmq publisher confirms: %w", err)
	}

	p.conn = conn
	p.channel = channel
	return nil
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

	p.mu.Lock()
	defer p.mu.Unlock()

	var publishErr error
	for attempt := 0; attempt < 2; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if p.conn == nil || p.channel == nil || p.conn.IsClosed() || p.channel.IsClosed() {
			if err := p.connectLocked(); err != nil {
				publishErr = err
				continue
			}
		}
		if err := p.publishConfirmed(ctx, routingKey, body); err == nil {
			return nil
		} else {
			publishErr = err
			publishErr = errors.Join(publishErr, p.closeLocked())
		}
	}

	return fmt.Errorf("publish otp message: %w", publishErr)
}

func (p *RabbitMQPublisher) publishConfirmed(ctx context.Context, routingKey string, body []byte) error {
	confirmation, err := p.channel.PublishWithDeferredConfirmWithContext(
		ctx,
		p.exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
			Timestamp:    time.Now(),
		},
	)
	if err != nil {
		return err
	}
	if confirmation == nil {
		return errors.New("rabbitmq publisher confirmation is unavailable")
	}
	confirmCtx, cancel := context.WithTimeout(ctx, rabbitPublishConfirmTimeout)
	defer cancel()
	acknowledged, err := confirmation.WaitContext(confirmCtx)
	if err != nil {
		return err
	}
	if !acknowledged {
		return errors.New("rabbitmq rejected published message")
	}
	return nil
}

func (p *RabbitMQPublisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closeLocked()
}

func (p *RabbitMQPublisher) closeLocked() error {
	var closeErr error
	if p.channel != nil && !p.channel.IsClosed() {
		closeErr = p.channel.Close()
	}
	if p.conn != nil && !p.conn.IsClosed() {
		closeErr = errors.Join(closeErr, p.conn.Close())
	}
	p.channel = nil
	p.conn = nil
	return closeErr
}
