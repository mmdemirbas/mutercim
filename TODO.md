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

- Even if the layout tool is disabled, we still need to know the layout information for the later steps. In that case the AI tool should try to infer this information. Or maybe it should try always but we prefer the layout tool's output when it is enabled.

- I NEED TO SKİP OCR BUT ONLY RUN LAYOUT TOOL FOR THE LAYOUT INFORMATION
