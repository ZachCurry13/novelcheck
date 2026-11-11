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
const CoverPrompt = `This picture shows a book: a photo of its cover, spine or back taken in a shop or at home, or a
screenshot of a page about it (a shop listing, a review, a post, an e-reader screen).
The picture may be sideways or upside down: read it anyway.
Read the book's title and author exactly as they are printed or written in the picture.
- The title is the book's own name, usually the largest words on a cover. It is not the series name, a line
  like "Book 2" or "A Novel", a slogan, a review quote, "New York Times bestseller", the publisher, or a shop's
  buttons and menus.
- The author is the name printed as the writer. Copy it letter for letter; never fill in or correct an author
  from memory, and never use the name of someone quoted praising the book.
- If the picture shows several books, read the one that is largest or in the middle.
- Never guess a title you cannot read. Do not answer from what the cover art looks like.
Also copy the ISBN if one is printed (digits only); never make one up.
Reply with JSON only: {"title": "...", "author": "...", "isbn": "..."}.
Use "" for anything you can't read. If the picture doesn't show a book, reply {"title": ""}.`
