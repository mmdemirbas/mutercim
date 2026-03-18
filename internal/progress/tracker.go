package progress

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
)

// PhaseName identifies a processing phase.
type PhaseName string

const (
	PhaseExtract   PhaseName = "extract"
	PhaseEnrich    PhaseName = "enrich"
	PhaseTranslate PhaseName = "translate"
	PhaseCompile   PhaseName = "compile"
)

// PhaseState tracks the state of a single phase.
type PhaseState struct {
	Completed []int  `json:"completed"`
	Failed    []int  `json:"failed"`
	Pending   []int  `json:"pending,omitempty"`
	LastRun   string `json:"last_run,omitempty"`
}

// State represents the full progress state.
type State struct {
	BookID     string                    `json:"book_id"`
	TotalPages int                       `json:"total_pages"`
	Phases     map[PhaseName]*PhaseState `json:"phases"`
}

// Tracker manages progress state with atomic saves.
type Tracker struct {
	path  string
	state *State
	mu    sync.Mutex
}

// NewTracker creates a tracker that reads/writes from the given path.
func NewTracker(path string) *Tracker {
	return &Tracker{
		path: path,
		state: &State{
			Phases: make(map[PhaseName]*PhaseState),
		},
	}
}

// Load reads progress state from disk. If the file doesn't exist,
// starts with empty state.
func (t *Tracker) Load() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	data, err := os.ReadFile(t.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read progress: %w", err)
	}

	if len(data) == 0 || string(data) == "{}\n" || string(data) == "{}" {
		return nil
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("unmarshal progress: %w", err)
	}
	if state.Phases == nil {
		state.Phases = make(map[PhaseName]*PhaseState)
	}
	t.state = &state
	return nil
}

// Save atomically writes progress state to disk.
func (t *Tracker) Save() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	data, err := json.MarshalIndent(t.state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal progress: %w", err)
	}

	tmpPath := t.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write progress tmp: %w", err)
	}
	if err := os.Rename(tmpPath, t.path); err != nil {
		return fmt.Errorf("rename progress: %w", err)
	}
	return nil
}

// State returns a copy of the current state.
func (t *Tracker) State() State {
	t.mu.Lock()
	defer t.mu.Unlock()
	// Shallow copy is fine for display purposes
	return *t.state
}

// MarkCompleted marks a page as completed for the given phase.
func (t *Tracker) MarkCompleted(phase PhaseName, page int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	ps := t.ensurePhase(phase)
	if !contains(ps.Completed, page) {
		ps.Completed = append(ps.Completed, page)
		sort.Ints(ps.Completed)
	}
	ps.Failed = removeInt(ps.Failed, page)
	ps.Pending = removeInt(ps.Pending, page)
}

// MarkFailed marks a page as failed for the given phase.
func (t *Tracker) MarkFailed(phase PhaseName, page int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	ps := t.ensurePhase(phase)
	if !contains(ps.Failed, page) {
		ps.Failed = append(ps.Failed, page)
		sort.Ints(ps.Failed)
	}
	ps.Pending = removeInt(ps.Pending, page)
}

// SetTotalPages sets the total page count.
func (t *Tracker) SetTotalPages(n int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.state.TotalPages = n
}

// SetBookID sets the book identifier.
func (t *Tracker) SetBookID(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.state.BookID = id
}

func (t *Tracker) ensurePhase(phase PhaseName) *PhaseState {
	ps, ok := t.state.Phases[phase]
	if !ok {
		ps = &PhaseState{}
		t.state.Phases[phase] = ps
	}
	return ps
}

func contains(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func removeInt(s []int, v int) []int {
	result := s[:0]
	for _, x := range s {
		if x != v {
			result = append(result, x)
		}
	}
	return result
}
