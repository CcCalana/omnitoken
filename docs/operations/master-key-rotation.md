# Master Key and Credential Operations

## v1 master key source

The gateway, admin service, and credential seed command prefer `OMNITOKEN_MASTER_KEY_FILE`. The file must contain a 32-byte key encoded as 64 hex characters. `OMNITOKEN_MASTER_KEY` remains a local-dev fallback for tests and one-off smoke runs.

For v1, OmniToken relies on a mounted secret file plus process memory lifetime. It does not unlink the mounted file or zeroize Go heap memory after startup. Production rotation is:

1. Mount a new master-key secret file.
2. Re-encrypt upstream credentials with that key.
3. Restart `credential-seed` if the seeded Ark keys changed.
4. Restart both `admin` and `gateway` so they read the same new key.

KMS-backed wrapping is vNext scope.

## Credential reload model

The v1 gateway decrypts upstream credentials at startup and refreshes the in-memory selector through `OMNITOKEN_CREDENTIAL_POLL_INTERVAL` polling, default `30s`. Set the interval to `0` to disable hot reload.

Adding a provider means adding `OMNITOKEN_<PROVIDER>_KEYS_*` plus `OMNITOKEN_<PROVIDER>_BASE_URL` and re-running `credential-seed`; it does not require a different master-key file.

Admin-created credentials are encrypted by the admin process and decrypted by the gateway, so both services must receive the same master key. KMS-backed wrapping remains v1.1+ scope.
