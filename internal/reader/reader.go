package reader

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/mmdemirbas/mutercim/internal/apiclient"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/provider"
)

// readResponse matches the JSON schema returned by the AI model.
type readResponse struct {
	PageNumber *int           `json:"page_number"`
	Header     *model.Header  `json:"header"`
	Entries    []model.Entry  `json:"entries"`
	Footnotes  []footnoteResp `json:"footnotes"`
	PageFooter string         `json:"page_footer"`
	Warnings   []string       `json:"warnings"`
}

// footnoteResp matches the footnote format in the read prompt schema.
type footnoteResp struct {
	EntryNumbers []int    `json:"entry_numbers"`
	ArabicText   string   `json:"arabic_text"`
	SourceCodes  []string `json:"source_codes"`
}

// Reader reads structured data from page images using an AI provider.
type Reader struct {
	provider provider.Provider
	logger   *slog.Logger
}

// NewReader creates a new Reader.
func NewReader(p provider.Provider, logger *slog.Logger) *Reader {
	if logger == nil {
		logger = slog.Default()
	}
	return &Reader{provider: p, logger: logger}
}

// ReadPage processes a single page image and returns a ReadPage.
func (r *Reader) ReadPage(ctx context.Context, image []byte, pageNum int, modelName string) (*model.ReadPage, error) {
	userPrompt := BuildUserPrompt()

	r.logger.Info("reading page", "page", pageNum)

	rawResponse, err := r.provider.ReadFromImage(ctx, image, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("read page %d: %w", pageNum, err)
	}

	jsonStr, err := apiclient.ExtractJSON(rawResponse)
	if err != nil {
		return nil, fmt.Errorf("parse JSON from response for page %d: %w", pageNum, err)
	}

	var resp readResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal read response for page %d: %w", pageNum, err)
	}

	page := &model.ReadPage{
		Version:       "1.0",
		PageNumber:    pageNum,
		ReadModel:     modelName,
		ReadTimestamp: time.Now().UTC().Format(time.RFC3339),
		Header:        resp.Header,
		Entries:       resp.Entries,
		Footnotes:     convertFootnotes(resp.Footnotes),
		PageFooter:    resp.PageFooter,
		RawText:       rawResponse,
		ReadWarnings:  resp.Warnings,
	}

	// Override page number from AI response if present
	if resp.PageNumber != nil {
		page.PageNumber = *resp.PageNumber
	}

	return page, nil
}

func convertFootnotes(resps []footnoteResp) []model.Footnote {
	footnotes := make([]model.Footnote, len(resps))
	for i, r := range resps {
		footnotes[i] = model.Footnote{
			EntryNumbers: r.EntryNumbers,
			ArabicText:   r.ArabicText,
			SourceCodes:  r.SourceCodes,
		}
	}
	return footnotes
}
