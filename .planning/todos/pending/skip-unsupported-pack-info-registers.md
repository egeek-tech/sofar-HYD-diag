---
title: Skip unsupported PackInfoProbes registers (0x9104-0x9126)
area: bms
priority: low
created: 2026-04-11
source: Phase 6 UAT — live hardware testing
---

## Description

Suppress or skip `PackInfoProbes` (0x9104-0x9126) read when BMS returns illegal address error (Modbus exception 0x02). These extended registers don't exist on this hardware.

## Context

During Phase 6 UAT, pack drill-down works correctly but produces error spam in logs:
```
level=ERROR msg="modbus operation failed" component=broker error="exception: func=0x83 err=0x02"
```

The error is at register 0x9104 ("Balanced Bus Voltage") — the start of a 35-register block read for extended pack info. The core pack data (RT probes 0x9044-0x907C) loads fine.

## Possible fixes

1. **Probe on first read, skip on error**: If 0x9104 returns illegal address, stop reading PackInfoProbes for the session
2. **Make PackInfoProbes optional**: Only read if a feature flag or auto-detection indicates support
3. **Silently swallow 0x02 errors**: Log at DEBUG level instead of ERROR for known-optional registers
