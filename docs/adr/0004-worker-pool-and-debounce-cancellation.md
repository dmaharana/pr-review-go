# In-Memory Worker Pool and In-Flight Debounce Cancellation

We decided to process webhooks asynchronously using an in-memory worker pool with immediate `202 Accepted` HTTP responses, combined with a per-PR cancellation registry. Webhook endpoints enforce strict timeouts (10-20s), while AI review takes longer. The cancellation registry cancels in-flight review contexts via `context.WithCancel()` when a newer commit is pushed to the same pull request, preventing duplicate comments and wasted LLM token spend.
