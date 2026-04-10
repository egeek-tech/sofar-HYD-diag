package register

// ProbeGroup organizes related probes into a named group with an optional layout hint.
// Layout "column" indicates the group should render side-by-side with other column groups.
// Empty layout means full-width.
// Type controls widget dispatch: "" = standard data rows, "bitmap" = bitmap widget,
// "protection" = protection/alarm card.
type ProbeGroup struct {
	Name   string
	Probes []Probe
	Layout string // "" = full width, "column" = side-by-side
	Type   string // "" = standard data rows, "bitmap" = bitmap widget, "protection" = protection/alarm card
}
