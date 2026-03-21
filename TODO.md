# DOCUMENTATION

- Separate development docs (SPEC, phases etc) from actual usage documentation.

- Provide AR/TR/ZH translations for docs

- Support multiple app languages (help messages, logs etc)? (ar/tr/zh)

# NEXT

- Allow prompt customization by user to teach adab to the tool:
  Custom prompt & âdâb: Three-layer prompt injection (built-in adab.md embedded in binary +
  workspace knowledge/prompt.md per book + config extra_prompt inline). Built-in ships with Islamic
  scholarly etiquette (salawat, honorifics, "Allah" not "Tanrı"). Knowledge YAMLs hold mappings,
  prompt files hold behavioral rules. Applies to both read and translate phases.

- Support fixing / completing tashkeels optionally in the arabic script.

- Dependencies? Single-binary?
- Do not require any external dependencies. Only docker. pandoc also should be used by docker. If
  docker is not available, or not running, we can try to use pandoc and latex directly installed on
  the host system.

- Windows compatibility?

- Optimize token usage
    * short JSON keys
    * shorter system prompts
