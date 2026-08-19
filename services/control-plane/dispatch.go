package controlplane

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	DefaultJobStream  = "FLIGHTCHECK_JOBS_V1"
	DefaultJobSubject = "flightcheck.jobs.evaluate"
)

type Publisher interface {
	Publish(context.Context, string, string, []byte) error
}

type NATSPublisher struct {
	connection *nats.Conn
	stream     jetstream.JetStream
}

func NewNATSPublisher(ctx context.Context, serverURL, streamName, subject string) (*NATSPublisher, error) {
	connection, err := nats.Connect(serverURL,
		nats.Name("fhir-flightcheck-control-plane"),
		nats.Timeout(5*time.Second),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("connect NATS: %w", err)
	}
	js, err := jetstream.New(connection)
	if err != nil {
		connection.Close()
		return nil, fmt.Errorf("open JetStream: %w", err)
	}
	if streamName == "" {
		streamName = DefaultJobStream
	}
	if subject == "" {
		subject = DefaultJobSubject
	}
	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name: streamName, Description: "FHIR Flightcheck evaluator jobs v1",
		Subjects: []string{subject}, Retention: jetstream.LimitsPolicy,
		Storage: jetstream.FileStorage, MaxAge: 7 * 24 * time.Hour,
		Duplicates: 24 * time.Hour,
	})
	if err != nil {
		connection.Close()
		return nil, fmt.Errorf("bootstrap JetStream stream: %w", err)
	}
	return &NATSPublisher{connection: connection, stream: js}, nil
}

func (p *NATSPublisher) Publish(ctx context.Context, subject, messageID string, payload []byte) error {
	_, err := p.stream.Publish(ctx, subject, payload, jetstream.WithMsgID(messageID))
	if err != nil {
		return fmt.Errorf("publish JetStream job: %w", err)
	}
	return nil
}

func (p *NATSPublisher) Close() error {
	if err := p.connection.Drain(); err != nil {
		p.connection.Close()
		return err
	}
	return nil
}

type OutboxDispatcher struct {
	Repository Repository
	Publisher  Publisher
	Logger     *slog.Logger
	Interval   time.Duration
}

func (d *OutboxDispatcher) Run(ctx context.Context) {
	interval := d.Interval
	if interval <= 0 {
		interval = 250 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := d.DispatchOnce(ctx); err != nil && ctx.Err() == nil {
			d.logger().Error("outbox dispatch failed", "error", Redact(err.Error()))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (d *OutboxDispatcher) DispatchOnce(ctx context.Context) error {
	messages, err := d.Repository.ClaimOutbox(ctx, 32, 30*time.Second)
	if err != nil {
		return err
	}
	for _, message := range messages {
		if err := d.Publisher.Publish(ctx, message.Subject, message.ID, message.Payload); err != nil {
			delay := time.Duration(math.Min(60, math.Pow(2, float64(message.Attempts)))) * time.Second
			if markErr := d.Repository.MarkOutboxFailed(ctx, message.ID, time.Now().UTC().Add(delay)); markErr != nil {
				return fmt.Errorf("publish: %v; release outbox: %w", err, markErr)
			}
			d.logger().Warn("outbox publish deferred", "message_id", message.ID,
				"attempt", message.Attempts, "retry_in_seconds", int(delay.Seconds()))
			continue
		}
		if err := d.Repository.MarkOutboxPublished(ctx, message.ID); err != nil {
			return err
		}
	}
	return nil
}

func (d *OutboxDispatcher) logger() *slog.Logger {
	if d.Logger != nil {
		return d.Logger
	}
	return slog.Default()
}

func EncodeJob(job EvaluationJob) ([]byte, error) {
	return json.Marshal(job)
}
