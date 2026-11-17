# PR Review Domain Glossary

Domain model for the Go pull request review agent (`pr-review-go`).

## Language

**Pull Request**:
A proposed set of code changes submitted for review in a Git hosting provider.
_Avoid_: Merge Request, patchset, change request

**Review Finding**:
A discrete, actionable issue detected in changed code, specifying affected lines, problem description, and severity.
_Avoid_: Lint error, feedback item, bug report

**Code Suggestion**:
A concrete replacement diff for a specific line span in the pull request that fixes an identified finding.
_Avoid_: Patch, inline fix, snippet

**Review Verdict**:
The overall merge recommendation determined by the review (`no_concerns_found`, `needs_review`, `changes_required`).
_Avoid_: Status, approval score, recommendation

**Git Provider**:
An integration layer that interacts with a specific Git hosting service (GitHub SaaS, GitHub Enterprise Server, Bitbucket Cloud, Bitbucket Data Center, or local git).
_Avoid_: VCS, forge, git host

**Hook Event**:
An inbound HTTP webhook payload received from a Git Provider notifying the service of a pull request lifecycle event (opened, modified, reopened, commented).
_Avoid_: Webhook trigger, notification, callback

**Worker Pool**:
An in-memory bounded queue of background review jobs executed by worker goroutines to prevent webhook timeouts and control concurrency.
_Avoid_: Task runner, background thread

**Debounce Cancellation**:
The mechanism that cancels an in-flight review for a pull request when a newer commit is pushed to the same pull request before the previous review finishes.
_Avoid_: Kill job, abort task

**Diff Chunking**:
Partitioning a large pull request diff into coherent file groups across multiple concurrent LLM review requests when the diff exceeds the token budget.
_Avoid_: Diff splitting, batching

**Commit Status**:
A provider-native check status (GitHub Check Run or Bitbucket Commit Status) indicating whether the pull request passed the review criteria.
_Avoid_: CI build, test check
