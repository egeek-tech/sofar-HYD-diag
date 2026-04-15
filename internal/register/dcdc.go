package register

// DCDCGroups contains DCDC converter register definitions organized into ProbeGroups.
// From Sofar Modbus-G3 V1.38 section 5.7 -- DCDC data area 0x5000-0x530F.
// Only read-only (R) registers included; RW/W control registers are excluded
// (read-only diagnostic tool).
//
// Hardware-verified via tools/section-sweep (2026-04-15).
// All 25 DCDC registers (4 groups: System Info, Real-Time Data, Faults, Capacity)
// returned illegal data address (0x02) or timeout on real hardware.
// Entire section emptied -- all groups removed.
// See tools/section-sweep/results.json for the full sweep report.
var DCDCGroups = []ProbeGroup{}
