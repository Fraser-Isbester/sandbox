package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	chatv1 "github.com/fraser-isbester/sandbox/chat/gen/chat/v1"
)

type ConversationStore struct {
	dataDir string
}

func NewConversationStore(dataDir string) *ConversationStore {
	return &ConversationStore{dataDir: dataDir}
}

func (s *ConversationStore) Save(conv *chatv1.Conversation) error {
	path := filepath.Join(s.dataDir, fmt.Sprintf("conv-%s.ndjson", conv.Id))
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening conversation file: %w", err)
	}
	defer f.Close()

	for _, msg := range conv.Messages {
		if err := json.NewEncoder(f).Encode(msg); err != nil {
			return fmt.Errorf("encoding message: %w", err)
		}
	}
	return nil
}

func (s *ConversationStore) Load(id string) (*chatv1.Conversation, error) {
	path := filepath.Join(s.dataDir, fmt.Sprintf("conv-%s.ndjson", id))
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening conversation file: %w", err)
	}
	defer f.Close()

	var messages []*chatv1.Message
	decoder := json.NewDecoder(f)
	for decoder.More() {
		var msg chatv1.Message
		if err := decoder.Decode(&msg); err != nil {
			return nil, fmt.Errorf("decoding message: %w", err)
		}
		messages = append(messages, &msg)
	}

	return &chatv1.Conversation{
		Id:       id,
		Messages: messages,
	}, nil
}
