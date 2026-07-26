package worker

import (
	"context"
	"encoding/json"
	"errors"
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
	if err := channel.Confirm(false); err != nil {
		return fmt.Errorf("enable publisher confirms: %w", err)
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
			c.handleDelivery(ctx, channel, delivery)
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

	retryDelayMillis := c.cfg.RetryDelay.Milliseconds()
	if _, err := channel.QueueDeclare(
		c.cfg.RetryQueue,
		true,
		false,
		false,
		false,
		amqp.Table{
			"x-message-ttl":             retryDelayMillis,
			"x-dead-letter-exchange":    c.cfg.Exchange,
			"x-dead-letter-routing-key": c.cfg.RetryKey,
		},
	); err != nil {
		return fmt.Errorf("declare retry queue: %w", err)
	}
	if _, err := channel.QueueDeclare(c.cfg.DeadQueue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare dead-letter queue: %w", err)
	}

	for _, key := range []string{c.cfg.RegisterKey, c.cfg.ResetKey, c.cfg.RetryKey} {
		if err := channel.QueueBind(queue.Name, key, c.cfg.Exchange, false, nil); err != nil {
			return fmt.Errorf("bind queue with routing key %s: %w", key, err)
		}
	}

	if err := channel.Qos(c.cfg.PrefetchCount, 0, false); err != nil {
		return fmt.Errorf("set qos: %w", err)
	}
	return nil
}

func (c *Consumer) handleDelivery(ctx context.Context, channel *amqp.Channel, delivery amqp.Delivery) {
	var msg OTPMessage
	if err := json.Unmarshal(delivery.Body, &msg); err != nil {
		c.logger.Error("discard invalid otp message", "error", err)
		_ = delivery.Nack(false, false)
		return
	}

	subject, body := buildOTPEmail(msg)
	if err := c.mailer.SendOTP(ctx, msg.Email, subject, body); err != nil {
		c.logger.Error("send otp email failed", "email", msg.Email, "type", msg.Type, "error", err)
		retries := messageRetryCount(delivery.Headers)
		targetQueue := c.cfg.RetryQueue
		if retries >= c.cfg.MaxRetries {
			targetQueue = c.cfg.DeadQueue
		}
		if publishErr := c.republish(ctx, channel, delivery, targetQueue, retries+1); publishErr != nil {
			c.logger.Error("republish failed email", "email", msg.Email, "type", msg.Type, "error", publishErr)
			_ = delivery.Nack(false, true)
			return
		}
		if targetQueue == c.cfg.DeadQueue {
			c.logger.Error("moved failed email to dead-letter queue", "email", msg.Email, "type", msg.Type)
		}
		_ = delivery.Ack(false)
		return
	}

	c.logger.Info("sent otp email", "email", msg.Email, "type", msg.Type)
	_ = delivery.Ack(false)
}

func (c *Consumer) republish(
	ctx context.Context,
	channel *amqp.Channel,
	delivery amqp.Delivery,
	targetQueue string,
	retryCount int,
) error {
	headers := make(amqp.Table, len(delivery.Headers)+1)
	for key, value := range delivery.Headers {
		headers[key] = value
	}
	headers["x-email-retry-count"] = int64(retryCount)

	confirmation, err := channel.PublishWithDeferredConfirmWithContext(
		ctx,
		"",
		targetQueue,
		false,
		false,
		amqp.Publishing{
			Headers:      headers,
			ContentType:  delivery.ContentType,
			DeliveryMode: amqp.Persistent,
			Body:         delivery.Body,
			Timestamp:    time.Now(),
		},
	)
	if err != nil {
		return fmt.Errorf("publish to %s: %w", targetQueue, err)
	}
	if confirmation == nil {
		return errors.New("rabbitmq publisher confirmation is unavailable")
	}
	confirmCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	acknowledged, err := confirmation.WaitContext(confirmCtx)
	if err != nil {
		return fmt.Errorf("wait for retry publish confirmation: %w", err)
	}
	if !acknowledged {
		return errors.New("rabbitmq rejected retry message")
	}
	return nil
}

func messageRetryCount(headers amqp.Table) int {
	switch value := headers["x-email-retry-count"].(type) {
	case int32:
		return int(value)
	case int64:
		return int(value)
	case int:
		return value
	default:
		return 0
	}
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
