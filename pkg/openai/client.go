package openai

import (
	"bytes"
	"context"
	"crypto/sha1"
	"emperror.dev/errors"
	"encoding/json"
	"fmt"
	"github.com/andybalholm/brotli"
	"github.com/dgraph-io/badger/v4"
	"github.com/op/go-logging"
	oai "github.com/sashabaranov/go-openai"
	"io"
)

func NewClient(apiKey string, db *badger.DB, logger *logging.Logger) *Client {

	return &Client{
		client: oai.NewClient(apiKey),
		apiKey: apiKey,
		badger: db,
		logger: logger,
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
	client *oai.Client
	apiKey string
	badger *badger.DB
	logger *logging.Logger
}

func (c *Client) CreateEmbedding(input string, model oai.EmbeddingModel) (*oai.Embedding, error) {
	var key = []byte(fmt.Sprintf("embedding-%x", sha1.Sum([]byte(input+string(model)))))
	var result *oai.Embedding
	c.badger.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			c.logger.Infof("cache miss value for key %s", string(key))
			return nil
		}
		if err := item.Value(func(val []byte) error {
			br := brotli.NewReader(bytes.NewReader(val))
			jsonBytes, err := io.ReadAll(br)
			if err != nil {
				return errors.Wrapf(err, "cannot read value from item for key %s", string(key))
			}
			result = &oai.Embedding{}
			if err := json.Unmarshal(jsonBytes, &result); err != nil {
				return errors.Wrapf(err, "cannot unmarshal json for key %s", string(key))
			}
			c.logger.Infof("cache hit value for key %s", string(key))
			return nil
		}); err != nil {
			return errors.Wrapf(err, "cannot get value from item for key %s", string(key))
		}
		return nil
	})

	if result != nil {
		return result, nil
	}

	// Create an EmbeddingRequest for the user query
	queryReq := oai.EmbeddingRequest{
		Input: []string{input},
		Model: model,
	}
	queryResponse, err := c.client.CreateEmbeddings(context.Background(), queryReq)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create embedding")
	}
	if len(queryResponse.Data) == 0 {
		return nil, errors.Errorf("no embedding returned")
	}
	result = &queryResponse.Data[0]
	if err := c.badger.Update(func(txn *badger.Txn) error {
		jsonStr, err := json.Marshal(result)
		if err != nil {
			return errors.Wrapf(err, "cannot marshal json for key %s", string(key))
		}
		buf := &bytes.Buffer{}
		wr := brotli.NewWriter(buf)
		if _, err := wr.Write(jsonStr); err != nil {
			return errors.Wrapf(err, "cannot write value for key %s", string(key))
		}
		if err := wr.Close(); err != nil {
			return errors.Wrapf(err, "cannot close writer for key %s", string(key))
		}
		if err := txn.Set(key, buf.Bytes()); err != nil {
			return errors.Wrapf(err, "cannot set value for key %s", string(key))
		}
		return nil
	}); err != nil {
		return nil, errors.Wrapf(err, "cannot set value for key %s", string(key))
	}

	return result, nil
}
