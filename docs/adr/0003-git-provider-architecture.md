# Git Provider Abstraction and Implementation Separation

We decided to define a unified `provider.GitProvider` interface and implement dedicated packages for GitHub (covering both SaaS and Enterprise Server via configurable BaseURL), Bitbucket Cloud (REST 2.0 API), Bitbucket Data Center (REST 1.0 API), and Local Git / Plain Diff. Bitbucket Cloud and Data Center have fundamentally different API contracts, authentication schemes, and comment formats; keeping them strictly separate prevents brittle conditional logic.
