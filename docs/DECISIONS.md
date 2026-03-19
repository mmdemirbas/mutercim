# Decisions & Overrides

Anything here overrides SPEC.md. The codebase is the source of truth.

## CLI Command Names
- extract → read
- enrich → solve
- translate → translate
- compile → write
- run → make

## Directory Names
- cache/ → midstate/

## Gemini Model
- Default: gemini-2.5-flash-lite (not gemini-2.0-flash)
- Rate limit: 10 RPM (not 14)