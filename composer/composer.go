package composer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/samber/lo"
	"github.com/samgozman/go-fin-feed/journalist"
	"github.com/sashabaranov/go-openai"
)

type OpenAiClientInterface interface {
	CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (response openai.ChatCompletionResponse, error error)
}

type Composer struct {
	OpenAiClient OpenAiClientInterface
	Config       *Config
}

func NewComposer(oaiToken string) *Composer {
	return &Composer{OpenAiClient: openai.NewClient(oaiToken), Config: DefaultConfig()}
}

func (c *Composer) Compose(ctx context.Context, news journalist.NewsList) ([]*ComposedNews, error) {
	// Filter out news that are not from today
	var todayNews journalist.NewsList = lo.Filter(news, func(n *journalist.News, _ int) bool {
		return n.Date.Day() == time.Now().Day()
	})

	if len(todayNews) == 0 {
		return nil, nil
	}

	// Convert news to JSON
	jsonNews, err := todayNews.ToContentJSON()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("[Compose] error in NewsList.ToContentJSON: %s", err))
	}

	// Compose news
	resp, err := c.OpenAiClient.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo1106,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: c.Config.ComposePrompt,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: jsonNews,
				},
			},
			Temperature:      1,
			MaxTokens:        2048,
			TopP:             1,
			FrequencyPenalty: 0,
			PresencePenalty:  0,
		},
	)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("[Compose] error in OpenAiClient.CreateChatCompletion: %s", err))
	}

	var fullComposedNews []*ComposedNews
	err = json.Unmarshal([]byte(resp.Choices[0].Message.Content), &fullComposedNews)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("[Compose] error in json.Unmarshal: %s for object: %s", err, resp.Choices[0].Message.Content))
	}

	return fullComposedNews, nil
}

type ComposedNews struct {
	ID       string   `json:"id"`
	Text     string   `json:"text"`
	Tickers  []string `json:"tickers"`  // tickers mentioned or/and related to the news
	Markets  []string `json:"markets"`  // US/EU/Asia stocks, bonds, commodities, housing, etc.
	Hashtags []string `json:"hashtags"` // hashtags related to the news (#inflation, #fed, #buybacks, etc.)
}
