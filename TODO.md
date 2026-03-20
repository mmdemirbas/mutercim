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

- Remove progress.json.

- Colorful help output

- If a file is already exists, skip that step for this file. For example, if a page is already read,
  do not try to re-read it unless the --force option provided. It should be skipped. So if the input
  is processed once for all pages succesfully, subsequent runs will be instantly skipping all pages
  in all steps and report that the operation succeeded. I think the only exception to this is the '
  write' step.

- Remove page prefix from generated files. Just use numbers (padded).

- Dashboard is not stable. It should update the output, not re-write it. The mechanism should be
  rewrite but user should experience it like in-place update.

- Arabic output is wrong! Both direction and letter combination are wrong.

- Enrich schema documentation. Make purpose of each field clear. For example, where the "title"
  and "author" used, etc.

- Allow prompt customization by user to teach adab to the tool:
  Custom prompt & âdâb: Three-layer prompt injection (built-in adab.md embedded in binary +
  workspace knowledge/prompt.md per book + config extra_prompt inline). Built-in ships with Islamic
  scholarly etiquette (salawat, honorifics, "Allah" not "Tanrı"). Knowledge YAMLs hold mappings,
  prompt files hold behavioral rules. Applies to both read and translate phases.

- Support fixing / completing tashkeels optionally in the arabic script.

