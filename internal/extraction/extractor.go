package extraction

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

// extractionResponse matches the JSON schema returned by the AI model.
type extractionResponse struct {
	PageNumber *int           `json:"page_number"`
	Header     *model.Header  `json:"header"`
	Entries    []model.Entry  `json:"entries"`
	Footnotes  []footnoteResp `json:"footnotes"`
	PageFooter string         `json:"page_footer"`
	Warnings   []string       `json:"warnings"`
}

// footnoteResp matches the footnote format in the extraction prompt schema.
type footnoteResp struct {
	EntryNumbers []int    `json:"entry_numbers"`
	ArabicText   string   `json:"arabic_text"`
	SourceCodes  []string `json:"source_codes"`
}

// Extractor extracts structured data from page images using an AI provider.
type Extractor struct {
	provider provider.Provider
	logger   *slog.Logger
}

// NewExtractor creates a new Extractor.
func NewExtractor(p provider.Provider, logger *slog.Logger) *Extractor {
	if logger == nil {
		logger = slog.Default()
	}
	return &Extractor{provider: p, logger: logger}
}

// ExtractPage processes a single page image and returns an ExtractedPage.
func (e *Extractor) ExtractPage(ctx context.Context, image []byte, pageNum int, sectionType, modelName string) (*model.ExtractedPage, error) {
	userPrompt := BuildUserPrompt(sectionType)

	e.logger.Info("extracting page", "page", pageNum, "section_type", sectionType)

	rawResponse, err := e.provider.ExtractFromImage(ctx, image, extractionSystemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("extract page %d: %w", pageNum, err)
	}

	jsonStr, err := apiclient.ExtractJSON(rawResponse)
	if err != nil {
		return nil, fmt.Errorf("extract JSON from response for page %d: %w", pageNum, err)
	}

	var resp extractionResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal extraction response for page %d: %w", pageNum, err)
	}

	page := &model.ExtractedPage{
		Version:             "1.0",
		PageNumber:          pageNum,
		SectionType:         sectionType,
		ExtractionModel:     modelName,
		ExtractionTimestamp: time.Now().UTC().Format(time.RFC3339),
		Header:              resp.Header,
		Entries:             resp.Entries,
		Footnotes:           convertFootnotes(resp.Footnotes),
		PageFooter:          resp.PageFooter,
		RawText:             rawResponse,
		ExtractionWarnings:  resp.Warnings,
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
