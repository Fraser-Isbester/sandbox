package llm

import (
	"github.com/anthropics/anthropic-sdk-go"
)

type Client struct {
	client *anthropic.Client
}

func NewClient() (*Client, error) {

	client := anthropic.NewClient()
	return &Client{client: client}, nil
}
