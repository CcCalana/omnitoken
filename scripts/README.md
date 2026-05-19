# Scripts

Reserved for repeatable local development, migration, and verification helpers.

`dev.ps1` mirrors the core Makefile targets for Windows machines without `make`.

`v1_integration.py` runs the T-INT control-plane smoke: admin/viewer login,
read tabs, viewer 403 on quota PATCH, admin virtual-key creation, and optional
real upstream chat when `OMNITOKEN_RUN_REAL_UPSTREAM=1` is set.
