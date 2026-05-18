package audit

import "expvar"

// AuditRecordFailuresTotal is a monotonic counter, never decremented.
var AuditRecordFailuresTotal = expvar.NewInt("omt_audit_record_failures_total")
