# Third-Party Licenses

This ledger tracks Go module dependencies added to `go.mod`. Weak copyleft
entries are allowed only as transitive dependencies and must not be forked or
modified in this repository.

| Module | Version | Direct | License | Notes |
| --- | --- | --- | --- | --- |
| `github.com/golang-migrate/migrate/v4` | `v4.18.3` | Yes | MIT | 未 fork、未修改源文件。 |
| `github.com/google/uuid` | `v1.6.0` | Yes | BSD-3-Clause | 未 fork、未修改源文件。 |
| `github.com/hashicorp/errwrap` | `v1.1.0` | No | MPL-2.0 | Transitive only; 未 fork、未修改源文件。 |
| `github.com/hashicorp/go-multierror` | `v1.1.1` | No | MPL-2.0 | Transitive only; 未 fork、未修改源文件。 |
| `github.com/lib/pq` | `v1.10.9` | Yes | MIT | 未 fork、未修改源文件。 |
| `go.uber.org/atomic` | `v1.7.0` | No | MIT | 未 fork、未修改源文件。 |

C-3 guardrail: do not modify MPL-2.0 package sources in `vendor/` or in the
module cache. If a future patch is needed, stop and propose before changing the
dependency posture.
