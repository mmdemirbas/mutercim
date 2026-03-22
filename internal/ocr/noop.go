package ocr

import "context"

// NoopTool is a no-op OCR tool used when OCR is disabled.
type NoopTool struct{}

// Name returns an empty string indicating no OCR tool.
func (n *NoopTool) Name() string { return "" }

// Start is a no-op.
func (n *NoopTool) Start(_ context.Context) error { return nil }

// Stop is a no-op.
func (n *NoopTool) Stop(_ context.Context) error { return nil }

// IsReady always returns false — the no-op tool is never ready.
func (n *NoopTool) IsReady(_ context.Context) bool { return false }

// RecognizeRegions returns ErrDisabled.
func (n *NoopTool) RecognizeRegions(_ context.Context, _ string, _ []RegionInput) (*Result, error) {
	return nil, ErrDisabled
}

// RecognizeFullPage returns ErrDisabled.
func (n *NoopTool) RecognizeFullPage(_ context.Context, _ string) (*Result, error) {
	return nil, ErrDisabled
}
