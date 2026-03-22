- Allow prompt customization by user to teach adab to the tool:
  Custom prompt & âdâb: Three-layer prompt injection (built-in adab.md embedded in binary +
  workspace knowledge/prompt.md per book + config extra_prompt inline). Built-in ships with Islamic
  scholarly etiquette (salawat, honorifics, "Allah" not "Tanrı"). Knowledge YAMLs hold mappings,
  prompt files hold behavioral rules. Applies to both read and translate phases.

- Optimize token usage
    * short JSON keys
    * shorter system prompts

- Support fixing / completing tashkeels optionally in the arabic script.

- Provide AR/TR/ZH translations for docs
- Support multiple app languages (help messages, logs etc)? (ar/tr/zh)

- No report.json written in the steps other than the 'read' step.

- "write" > "expand_sources" property is tightly coupled to my hadith book sample. It conflicts with
  my general purpose translator tool vision.
* Instead, add a "source_expansion" step that can be optionally added after "translate" or "solve". It will take the translated output and expand it with source information (e.g. for hadith, add isnad and original text in arabic). This way it's decoupled from the writing/formatting logic and can be used in more contexts.  

