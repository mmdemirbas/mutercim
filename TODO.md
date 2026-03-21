- Remove built-in glossary and provide it as an example external file only.

- Allow prompt customization by user to teach adab to the tool:
  Custom prompt & âdâb: Three-layer prompt injection (built-in adab.md embedded in binary +
  workspace knowledge/prompt.md per book + config extra_prompt inline). Built-in ships with Islamic
  scholarly etiquette (salawat, honorifics, "Allah" not "Tanrı"). Knowledge YAMLs hold mappings,
  prompt files hold behavioral rules. Applies to both read and translate phases.

- Optimize token usage
    * short JSON keys
    * shorter system prompts

- Support fixing / completing tashkeels optionally in the arabic script.

- Windows compatibility?

- Provide AR/TR/ZH translations for docs
- Support multiple app languages (help messages, logs etc)? (ar/tr/zh)
