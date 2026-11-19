package textgen

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

// baseModel writes the session's site. routeModel writes every other path.
// Haiku 5.5 takes the same adaptive-thinking request as Sonnet 5, at low effort.
const baseModel = anthropic.ModelClaudeSonnet5
const routeModel anthropic.Model = "claude-haiku-5-5"

// cloudMaxTokens caps a response. Thinking tokens count against it, and adaptive
// thinking can spend more of the budget than the visible JSON does.
const cloudMaxTokens = 16000

type Client struct {
	cloud anthropic.Client
}

func NewClient() *Client {
	// anthropic.NewClient reads ANTHROPIC_API_KEY from the environment.
	return &Client{cloud: anthropic.NewClient()}
}

func modelFor(home bool) anthropic.Model {
	if home {
		return baseModel
	}
	return routeModel
}

func effortFor(home bool) anthropic.OutputConfigEffort {
	if home {
		return anthropic.OutputConfigEffortMedium
	}
	return anthropic.OutputConfigEffortLow
}

func (c *Client) Complete(ctx context.Context, home bool, instructions, input string) (string, error) {
	msg, err := c.cloud.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     modelFor(home),
		MaxTokens: cloudMaxTokens,
		Thinking: anthropic.ThinkingConfigParamUnion{
			OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{},
		},
		OutputConfig: anthropic.OutputConfigParam{Effort: effortFor(home)},
		System: []anthropic.TextBlockParam{{
			Text:         instructions,
			CacheControl: anthropic.NewCacheControlEphemeralParam(),
		}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(input)),
		},
	})
	if err != nil {
		return "", err
	}
	if msg == nil {
		return "", fmt.Errorf("claude returned no message")
	}

	var b strings.Builder
	for _, block := range msg.Content {
		if text, ok := block.AsAny().(anthropic.TextBlock); ok && text.Text != "" {
			b.WriteString(text.Text)
		}
	}
	if b.Len() == 0 {
		return "", fmt.Errorf("claude returned no text")
	}
	return b.String(), nil
}
