# Single Binary Architecture for CLI and Webhook Daemon

We decided to build `pr-review-go` as a single compiled Go binary containing both developer-facing CLI commands (`review`, `describe`, `ask`) and a persistent HTTP webhook daemon (`serve`). This eliminates multi-container deployment overhead, shares the exact same core review engine across local and CI/server workflows, and keeps deployment footprints to a single ~20MB static binary.
