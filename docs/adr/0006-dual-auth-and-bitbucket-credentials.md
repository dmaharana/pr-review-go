# GitHub Dual-Auth and Bitbucket Explicit Credentials

We decided to support both GitHub App authentication (App ID + Private Key) and Personal Access Tokens (PAT) for GitHub, and explicit configuration keys for Bitbucket Cloud (`app_password`) and Bitbucket Data Center (`token`). This provides enterprise security for automated webhooks while maintaining zero-friction local CLI developer usage.
