- Support multiple app languages (help messages, logs etc)? (ar/tr/zh)

- Add README
    * Document .env and .envrc autoload behaviour
    * Document prerequisites for development and usage (single-binary, or docker required? or
      optional?)

- Separate development docs (SPEC, phases etc) from actual usage documentation.

- Provide AR/TR/ZH translations for docs

- Optimize token usage
    * short JSON keys
    * shorter system prompts

- Dependencies? Single-binary?

- Support fixing / completing tashkeels optionally in the arabic script.

- Support on-demand output formats configs: 'mutercim write pdf' (pdf|md|tex|docx etc). Same can be support with 'mutercim make'. Even if the configured output formats are different, this command specifically build a pdf (or whatever output format specified in cli).

- What does 'validate' do? How important is it? Should we remove it?

- Allow adding to the system prompt, or a second prompt maybe (user prompt).