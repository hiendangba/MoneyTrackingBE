package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"email-service/internal/config"
	"email-service/internal/mailer"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Consumer struct {
	cfg    config.RabbitMQConfig
	mailer *mailer.Mailer
	logger *slog.Logger
}

type OTPMessage struct {
	Type             string `json:"type"`
	Email            string `json:"email"`
	FullName         string `json:"full_name,omitempty"`
	OTPCode          string `json:"otp_code"`
	ExpiresInSeconds int64  `json:"expires_in_seconds"`
}

func NewConsumer(cfg config.RabbitMQConfig, mailer *mailer.Mailer, logger *slog.Logger) *Consumer {
	return &Consumer{
		cfg:    cfg,
		mailer: mailer,
		logger: logger,
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	for {
		if err := c.consume(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			c.logger.Error("email consumer stopped", "error", err)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(c.cfg.ReconnectDelay):
		}
	}
}

func (c *Consumer) consume(ctx context.Context) error {
	conn, err := amqp.Dial(c.cfg.URL)
	if err != nil {
		return fmt.Errorf("dial rabbitmq: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	channel, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("open rabbitmq channel: %w", err)
	}
	defer func() {
		_ = channel.Close()
	}()

	if err := c.setupTopology(channel); err != nil {
		return err
	}

	deliveries, err := channel.Consume(
		c.cfg.Queue,
		"email-service",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("consume queue: %w", err)
	}

	c.logger.Info("email consumer started", "queue", c.cfg.Queue)
	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("rabbitmq deliveries channel closed")
			}
			c.handleDelivery(ctx, delivery)
		}
	}
}

func (c *Consumer) setupTopology(channel *amqp.Channel) error {
	if err := channel.ExchangeDeclare(
		c.cfg.Exchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("declare exchange: %w", err)
	}

	queue, err := channel.QueueDeclare(
		c.cfg.Queue,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("declare queue: %w", err)
	}

	for _, key := range []string{c.cfg.RegisterKey, c.cfg.ResetKey} {
		if err := channel.QueueBind(queue.Name, key, c.cfg.Exchange, false, nil); err != nil {
			return fmt.Errorf("bind queue with routing key %s: %w", key, err)
		}
	}

	if err := channel.Qos(c.cfg.PrefetchCount, 0, false); err != nil {
		return fmt.Errorf("set qos: %w", err)
	}
	return nil
}

func (c *Consumer) handleDelivery(ctx context.Context, delivery amqp.Delivery) {
	var msg OTPMessage
	if err := json.Unmarshal(delivery.Body, &msg); err != nil {
		c.logger.Error("discard invalid otp message", "error", err)
		_ = delivery.Nack(false, false)
		return
	}

	subject, body := buildOTPEmail(msg)
	if err := c.mailer.SendOTP(ctx, msg.Email, subject, body); err != nil {
		c.logger.Error("send otp email failed", "email", msg.Email, "type", msg.Type, "error", err)
		_ = delivery.Nack(false, true)
		return
	}

	c.logger.Info("sent otp email", "email", msg.Email, "type", msg.Type)
	_ = delivery.Ack(false)
}

func buildOTPEmail(msg OTPMessage) (string, string) {
	expiresIn := time.Duration(msg.ExpiresInSeconds) * time.Second
	switch msg.Type {
	case "register_otp":
		return "MoneyTracking registration OTP",
			fmt.Sprintf("Hello %s,\n\nYour MoneyTracking registration OTP is %s.\nThis code expires in %s.\n\nIf you did not request this, you can ignore this email.", msg.FullName, msg.OTPCode, expiresIn)
	case "reset_password_otp":
		return "MoneyTracking password reset OTP",
			fmt.Sprintf("Your MoneyTracking password reset OTP is %s.\nThis code expires in %s.\n\nIf you did not request this, you can ignore this email.", msg.OTPCode, expiresIn)
	default:
		return "MoneyTracking OTP",
			fmt.Sprintf("Your MoneyTracking OTP is %s.\nThis code expires in %s.", msg.OTPCode, expiresIn)
	}
}
