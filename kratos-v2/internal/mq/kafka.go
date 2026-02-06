package mq

import (
	"context"
	"encoding/json"
	"os"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/segmentio/kafka-go"
)

type TransferMessage struct {
	FileMD5      string `json:"file_md5"`
	CurLocation  string `json:"cur_location"`
	DestLocation string `json:"dest_location"`
}

type ThumbnailMessage struct {
	FileID   int64  `json:"file_id"`
	FilePath string `json:"file_path"`
	FileType string `json:"file_type"`
}

type TrashCleanupMessage struct {
	FileIDs []int64 `json:"file_ids"`
}

type Producer struct {
	writer *kafka.Writer
	log    *log.Helper
}

func NewProducer(logger log.Logger) *Producer {
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		brokers = "localhost:9092"
	}

	return &Producer{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(brokers),
			Balancer: &kafka.LeastBytes{},
		},
		log: log.NewHelper(logger),
	}
}

func (p *Producer) SendTransferMessage(ctx context.Context, topic string, msg *TransferMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return p.writer.WriteMessages(ctx, kafka.Message{
		Topic: topic,
		Value: data,
	})
}

func (p *Producer) SendThumbnailMessage(ctx context.Context, topic string, msg *ThumbnailMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return p.writer.WriteMessages(ctx, kafka.Message{
		Topic: topic,
		Value: data,
	})
}

func (p *Producer) Close() error {
	return p.writer.Close()
}

type Consumer struct {
	reader *kafka.Reader
	log    *log.Helper
}

func NewConsumer(topic, groupID string, logger log.Logger) *Consumer {
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		brokers = "localhost:9092"
	}

	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: []string{brokers},
			Topic:   topic,
			GroupID: groupID,
		}),
		log: log.NewHelper(logger),
	}
}

func (c *Consumer) Consume(ctx context.Context, handler func([]byte) error) error {
	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			return err
		}

		if err := handler(msg.Value); err != nil {
			c.log.Errorf("failed to handle message: %v", err)
		}
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
