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


- Should continue trying next model instead of terminating the operation:
  09:07:07 INFO   reading page regions  page=156  strategy=local+ai
  09:07:08 ERROR  read failed  input=Anfas1  page=156  error=read page 156 regions: openrouter read: HTTP 404: 404 Not Found
  09:07:08 INFO   input read complete  input=Anfas1  completed=0  failed=1  skipped=0

- Add log level to the configuration file.
- Add '-f' short option to the '--force' option in cli.
- Add '-a' short option to  the '--auto' option in cli.
- Add '-l' short option to the '--log-level' option in cli.
- The default output directory is not './write'. It is current dir './'. Update it in the usage text, and any other similar place if there are any stale information.
- Add subcommand cli options (flags) for the configuration items defined in the config file for each step. Assign short options also by default.

- 'knowledge' should be a property of the 'translate' step (I think).

