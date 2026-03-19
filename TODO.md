DOCUMENTATION ======================================================================================

- Add README
    * Document .env and .envrc autoload behaviour
    * Document prerequisites for development and usage (single-binary, or docker required? or
      optional?)

- Separate development docs (SPEC, phases etc) from actual usage documentation.

- Provide AR/TR/ZH translations for docs

- Support multiple app languages (help messages, logs etc)? (ar/tr/zh)


OPTIMIZATION =======================================================================================

- Optimize token usage
    * short JSON keys
    * shorter system prompts


FEATURES ===========================================================================================

- The tool should be idempotent and skip already-processed pages by default. We should provide a force flag, so we can force it to re-process.

- Support fixing / completing tashkeels optionally in the arabic script.

- Allow adding to the system prompt, or a second prompt maybe (user prompt). 
  So we can teach some âdâb to the AI (Peygamberimiz, Efendimiz vs.)


SIMPLIFICATION =====================================================================================

- What does 'validate' do? How important is it? Should we remove it?

- Dependencies? Single-binary?
