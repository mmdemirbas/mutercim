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

- Add a flag to auto-execute missing previous steps if needed

- Support fixing / completing tashkeels optionally in the arabic script.

- Support on-demand output formats 'mutercim write pdf' (pdf|md|tex|docx etc). Same can be support with 'mutercim make'

- Refactor write section. Remove skip_pdf, and define pdf as a default output format. If it implies latex, that is ok.

IN PROGRESS:

- Docker errors should be explicit. If PDf requested but failed to be generated, this should be explicit in the output / console.
- When operation completed, a quick summary should be printed.
- We cannot hardcode the architecture in the docker file. This is a cross platform tool.
