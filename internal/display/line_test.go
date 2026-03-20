package display

import (
	"bytes"
	"strings"
	"testing"
)

func TestLineDisplayUpdate(t *testing.T) {
	var buf bytes.Buffer
	d := newLineDisplay(&buf)

	d.StartPhase(PhaseRead, "vol1", 100, "")
	d.Update(PageResult{
		Phase:     PhaseRead,
		Input:     "vol1",
		PageNum:   42,
		Total:     100,
		Completed: 42,
		Entries:   5,
		Footnotes: 3,
	})

	out := buf.String()
	if !strings.Contains(out, "[READ]") {
		t.Errorf("should contain [READ], got: %q", out)
	}
	if !strings.Contains(out, "42/100") {
		t.Errorf("should contain 42/100, got: %q", out)
	}
	if !strings.Contains(out, "5 entries") {
		t.Errorf("should contain '5 entries', got: %q", out)
	}
	if !strings.Contains(out, "3 footnotes") {
		t.Errorf("should contain '3 footnotes', got: %q", out)
	}
}

func TestLineDisplayUpdateError(t *testing.T) {
	var buf bytes.Buffer
	d := newLineDisplay(&buf)

	d.StartPhase(PhaseRead, "vol1", 100, "")
	d.Update(PageResult{
		Phase:   PhaseRead,
		Input:   "vol1",
		PageNum: 42,
		Total:   100,
		Err:     errTest,
	})

	out := buf.String()
	if !strings.Contains(out, "FAILED") {
		t.Errorf("should contain FAILED, got: %q", out)
	}
	if !strings.Contains(out, "test error") {
		t.Errorf("should contain error message, got: %q", out)
	}
}

var errTest = &testError{}

type testError struct{}

func (e *testError) Error() string { return "test error" }

func TestLineDisplayTranslateWithLang(t *testing.T) {
	var buf bytes.Buffer
	d := newLineDisplay(&buf)

	d.StartPhase(PhaseTranslate, "vol1", 50, "tr")
	d.Update(PageResult{
		Phase:     PhaseTranslate,
		Input:     "vol1",
		PageNum:   1,
		Total:     50,
		Completed: 1,
		Lang:      "tr",
	})

	out := buf.String()
	if !strings.Contains(out, "TRANS:tr") {
		t.Errorf("should contain TRANS:tr, got: %q", out)
	}
}

func TestLineDisplayFinishPhase(t *testing.T) {
	var buf bytes.Buffer
	d := newLineDisplay(&buf)

	d.StartPhase(PhaseRead, "vol1", 10, "")
	d.Update(PageResult{Phase: PhaseRead, Input: "vol1", Completed: 8, Failed: 2, Warnings: 3})

	buf.Reset()
	d.FinishPhase(PhaseRead, "vol1", "")
	out := buf.String()
	if !strings.Contains(out, "done") {
		t.Errorf("should contain 'done', got: %q", out)
	}
	if !strings.Contains(out, "8 completed") {
		t.Errorf("should contain '8 completed', got: %q", out)
	}
}

func TestLineDisplayFinishSummary(t *testing.T) {
	var buf bytes.Buffer
	d := newLineDisplay(&buf)

	d.StartPhase(PhaseRead, "vol1", 10, "")
	d.Update(PageResult{Phase: PhaseRead, Input: "vol1", Total: 10, Completed: 10})
	d.FinishPhase(PhaseRead, "vol1", "")

	buf.Reset()
	d.Finish()
	out := buf.String()
	if !strings.Contains(out, "10/10") {
		t.Errorf("Finish should show 10/10, got: %q", out)
	}
}

func TestIsTTYWithBuffer(t *testing.T) {
	var buf bytes.Buffer
	if IsTTY(&buf) {
		t.Error("bytes.Buffer should not be a TTY")
	}
}
