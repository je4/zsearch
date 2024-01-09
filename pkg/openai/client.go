package openai

import (
	"bytes"
	"emperror.dev/errors"
	"encoding/json"
	"github.com/dgraph-io/badger/v4"
	"github.com/op/go-logging"
	"io"
	"net/http"
	"strings"
)

func NewClient(baseURL, apiKey string, db *badger.DB, logger *logging.Logger) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey,
		badger:  db,
		logger:  logger,
	}
}

type EmbeddingResult struct {
	Vector []float64 `json:"embedding"`
	Index  int       `json:"index"`
	Object string    `json:"object"`
}

type EmbeddingRequest struct {
	Input          string `json:"input"`
	Model          string `json:"model"`
	EncodingFormat string `json:"encoding_format,omitempty"`
	User           string `json:"user,omitempty"`
}

type Client struct {
	apiKey  string
	baseURL string
	badger  *badger.DB
	logger  *logging.Logger
}

func (c *Client) CreateEmbedding(input, model string) (*EmbeddingResult, error) {
	client := &http.Client{}
	data, err := json.Marshal(&EmbeddingRequest{Input: input, Model: model})
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal json for embedding request")
	}
	req, err := http.NewRequest("POST", c.baseURL+"/v1/embeddings", bytes.NewBuffer(data))
	if err != nil {
		return nil, errors.Wrap(err, "cannot create request for embedding")
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+c.apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot post request to embedding")
	}
	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot read response from embedding")
	}
	resp.Body.Close()
	responseStruct := EmbeddingResult{}
	if err := json.Unmarshal(data, &responseStruct); err != nil {
		return nil, errors.Wrapf(err, "cannot unmarshal response  from embedding")
	}
	return nil, nil

}
