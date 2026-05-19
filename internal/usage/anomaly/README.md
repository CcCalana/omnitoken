# Usage Anomaly Package

Scans persisted usage events for coarse key-level spikes and emits sanitized WARN logs.
It does not block requests, write audit rows, or expose webhook/email delivery.
