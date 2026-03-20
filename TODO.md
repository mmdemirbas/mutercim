# OPTIMIZATION

- Optimize token usage
    * short JSON keys
    * shorter system prompts

# SIMPLIFICATION

- remove knowledge staged - promote flow. Let inferred knowledge to stay in its 'memory' folder.
  User could move them manually if they want.

- Dependencies? Single-binary?

- Simplify knowledge management. Should we move embedded knowledge to the example/template?
  Should we reduce number of knowledge files or unify their structures?

- Simplify overall architecture and code structure. I think there are unnecessary code layers.
  We can reduce number of files and abstractions. We can unify duplications. We can simplify
  code structure. We can remove unnecessary abstractions.

# DOCUMENTATION

- Add README
    * Document .env and .envrc autoload behaviour
    * Document prerequisites for development and usage (single-binary, or docker required? or
      optional?)

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

- Do not require any external dependencies. Only docker. pandoc also should be used by docker. If docker is not available, or not running, we can try to use pandoc and latex directly installed on the host system.
