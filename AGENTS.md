# Repository maintenance constraints

- Read CONTRIBUTING.md before changing branch or release workflows.
- Work from custom/main using feature/*, fix/* or sync/upstream-* branches.
- main and upstream/main are official upstream mirrors. Never add custom patches there.
- Merge official upstream changes into a branch based on custom/main; resolve conflicts explicitly and retain local patches. Do not replace the tree with ours/theirs.
- production accepts only a same-repository custom/main PR after required checks. No direct push, force push, deletion, upstream-to-production merge or deployment of an unmerged feature branch.
- Hotfixes go through custom/main before production. Verify first-output timeout, failover and edit-form behavior when touching gateway/account logic.
- Deploy a precise production SHA, preserve rollback, and record SHA/image digest and health verification. A merged PR does not prove deployment.
- Never commit credentials, runtime configuration, database dumps or customer data to this public repository.
