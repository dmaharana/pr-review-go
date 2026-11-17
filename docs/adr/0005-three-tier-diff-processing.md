# Three-Tier Diff Processing and Token Chunking

We decided to handle pull request diffs using a three-tier pipeline: automatic file filtering (ignoring lockfiles, generated files, and binaries), token budget inspection, and concurrent chunk fan-out using `errgroup.Group` when diffs exceed the model token window. This ensures complete review coverage of large pull requests without truncating important files or exceeding model context limits.
