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

- What is the purpose of the author field? Is it used anywhere? If not, then remove it.
- Enrich schema documentation. Make purpose of each field clear. For example, where the "title" used, etc.

- Dashboard is not stable. It should update the output, not re-write it. The mechanism should be
  rewrite but user should experience it like in-place update. Also the output is reporting 'tr' 3 times while it should report 'tr', 'en' and 'ar' according to the given input.

   ```
  ❯ mutercim all
    Book: Anfas1-demo — Unknown Author
   Input: Anfas1.pdf
   Langs: ar → ar, en, tr
  
         PAGES  ████████████████████ 6/6  ✓  0s
          READ  ░░░░░░░░░░░░░░░░░░░░ 0/6  ✓  11m
                ✗ 3 errors
    Book: Anfas1-demo — Unknown Author
   Input: Anfas1.pdf
   Langs: ar → ar, en, tr
  
         PAGES  ████████████████████ 6/6  ✓  0s
          READ  ░░░░░░░░░░░░░░░░░░░░ 0/6  ✓  11m
                ✗ 3 errors
         SOLVE  ░░░░░░░░░░░░░░░░░░░░ 0/3  ✓  0s
    TRANS [tr]  ░░░░░░░░░░░░░░░░░░░░ 0/3  ✓  0s
    TRANS [tr]  ░░░░░░░░░░░░░░░░░░░░ 0/3  ✓  0s
    TRANS [tr]  ░░░░░░░░░░░░░░░░░░░░ 0/3  ✓  0s
    WRITE [tr]  ████████████████████ 3/3  ✓  1s
    WRITE [tr]  ████████████████████ 3/3  ✓  1s
    WRITE [tr]  ████████████████████ 3/3  ✓  1s
  ```

- Arabic output is wrong! Both direction and letter combination looks wrong.

- Allow prompt customization by user to teach adab to the tool:
  Custom prompt & âdâb: Three-layer prompt injection (built-in adab.md embedded in binary +
  workspace knowledge/prompt.md per book + config extra_prompt inline). Built-in ships with Islamic
  scholarly etiquette (salawat, honorifics, "Allah" not "Tanrı"). Knowledge YAMLs hold mappings,
  prompt files hold behavioral rules. Applies to both read and translate phases.

- Support fixing / completing tashkeels optionally in the arabic script.

