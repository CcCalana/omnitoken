# Master Key and Credential Operations

## v1 master key source

The gateway and credential seed command prefer `OMNITOKEN_MASTER_KEY_FILE`. The file must contain a 32-byte key encoded as 64 hex characters. `OMNITOKEN_MASTER_KEY` remains a local-dev fallback for tests and one-off smoke runs.

For v1, OmniToken relies on a mounted secret file plus process memory lifetime. It does not unlink the mounted file or zeroize Go heap memory after startup. Production rotation is:

1. Mount a new master-key secret file.
2. Re-encrypt upstream credentials with that key.
3. Restart `credential-seed` if the seeded Ark keys changed.
4. Restart `gateway` so it reads the new key and reloads the credential pool.

KMS-backed wrapping is vNext scope.

## Credential reload model

The v1 gateway decrypts upstream credentials once at startup and keeps them in memory for routing. Direct database edits or seed changes do not affect a running gateway. After adding, disabling, or reseeding credentials, restart `gateway`.

Adding a provider means adding `OMNITOKEN_<PROVIDER>_KEYS_*` plus `OMNITOKEN_<PROVIDER>_BASE_URL` and re-running `credential-seed`; it does not require a different master-key file.

Do not expect SIGHUP, PG NOTIFY, or timed polling in v1; those reload paths belong with the v1.1 admin CRUD workflow.
