# Embedded Prompt Templates and Structured JSON Outputs

We decided to use Go standard library `embed.FS` with `text/template` for prompt generation and require structured JSON output from LLMs rather than replicating Python's Jinja2 + YAML approach. Modern LLMs support JSON schemas and constrained decoding reliably, and Go unmarshals directly into typed domain structs (`ReviewFinding`, `ReviewOutput`) with compile-time type safety.
