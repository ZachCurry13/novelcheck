package llm

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// ImageReader is a client that can look at a photo (for Check a book).
// Whether the chosen model can see images is only known by trying.
type ImageReader interface {
	ReadImage(ctx context.Context, model, prompt string, image []byte, mediaType string) (string, Usage, error)
}

// partsMessage is an OpenAI-style message with text and image parts.
type partsMessage struct {
	Role    string `json:"role"`
	Content []part `json:"content"`
}

type part struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

// ReadImage asks an OpenAI-compatible vision model (OpenAI, Gemini, Ollama
// llava/llama3.2-vision…) about a photo sent as a data: URL.
func (c *Client) ReadImage(ctx context.Context, model, prompt string, image []byte, mediaType string) (string, Usage, error) {
	if strings.TrimSpace(c.BaseURL) == "" || strings.TrimSpace(model) == "" {
		return "", Usage{}, errors.New("LLM base URL and model must be configured")
	}
	data := "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(image)
	return c.send(ctx, chatRequest{
		Model: model,
		Messages: []any{partsMessage{Role: "user", Content: []part{
			{Type: "text", Text: prompt},
			{Type: "image_url", ImageURL: &imageURL{URL: data}},
		}}},
		MaxTokens: 300,
	})
}

// ReadImage asks Claude about a photo.
func (c *AnthropicClient) ReadImage(ctx context.Context, model, prompt string, image []byte, mediaType string) (string, Usage, error) {
	if strings.TrimSpace(c.APIKey) == "" || strings.TrimSpace(model) == "" {
		return "", Usage{}, errors.New("Claude needs an Anthropic API key and a model")
	}
	opts := []option.RequestOption{option.WithAPIKey(c.APIKey)}
	if c.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(c.BaseURL))
	}
	client := anthropic.NewClient(opts...)
	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: 300,
		Messages: []anthropic.MessageParam{anthropic.NewUserMessage(
			anthropic.NewImageBlockBase64(mediaType, base64.StdEncoding.EncodeToString(image)),
			anthropic.NewTextBlock(prompt),
		)},
	})
	if err != nil {
		return "", Usage{}, err
	}
	usage := Usage{PromptTokens: int(resp.Usage.InputTokens), CompletionTokens: int(resp.Usage.OutputTokens)}
	var text strings.Builder
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			text.WriteString(t.Text)
		}
	}
	return text.String(), usage, nil
}

// CoverPrompt asks a vision model to identify the book in a photo.
const CoverPrompt = `This is a photo of a book (its cover, spine or back) taken in a shop or at home.
Read the book's title and author from it. Also copy the ISBN if one is printed (digits only).
Reply with JSON only: {"title": "...", "author": "...", "isbn": "..."}.
Use "" for anything you can't read. If the photo doesn't show a book, reply {"title": ""}.`
