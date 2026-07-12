package sorters

// ReadingOrder defines the primary and secondary text flow direction.
// This is critical for correctly calculating distances between blocks
// in different writing systems (e.g., vertical Chinese vs horizontal English).
type ReadingOrder int

const (
	// HorizontalLTR_TTB is left-to-right, top-to-bottom reading order.
	// Used by: English, most European languages, modern Chinese
	// Primary axis: Horizontal (left → right)
	// Secondary axis: Vertical (top → bottom)
	HorizontalLTR_TTB ReadingOrder = iota

	// HorizontalRTL_TTB is right-to-left, top-to-bottom reading order.
	// Used by: Arabic, Hebrew, Persian
	// Primary axis: Horizontal (right → left)
	// Secondary axis: Vertical (top → bottom)
	HorizontalRTL_TTB

	// VerticalTTB_RTL is top-to-bottom, right-to-left reading order.
	// Used by: Traditional Chinese, Japanese
	// Primary axis: Vertical (top → bottom)
	// Secondary axis: Horizontal (right → left)
	VerticalTTB_RTL

	// VerticalTTB_LTR is top-to-bottom, left-to-right reading order.
	// Used by: Mongolian
	// Primary axis: Vertical (top → bottom)
	// Secondary axis: Horizontal (left → right)
	VerticalTTB_LTR
)

// String returns a human-readable name for the reading order.
func (r ReadingOrder) String() string {
	switch r {
	case HorizontalLTR_TTB:
		return "Horizontal LTR, Top-to-Bottom"
	case HorizontalRTL_TTB:
		return "Horizontal RTL, Top-to-Bottom"
	case VerticalTTB_RTL:
		return "Vertical Top-to-Bottom, Right-to-Left"
	case VerticalTTB_LTR:
		return "Vertical Top-to-Bottom, Left-to-Right"
	default:
		return "Unknown"
	}
}

// IsHorizontal returns true if the primary reading direction is horizontal.
func (r ReadingOrder) IsHorizontal() bool {
	return r == HorizontalLTR_TTB || r == HorizontalRTL_TTB
}

// IsVertical returns true if the primary reading direction is vertical.
func (r ReadingOrder) IsVertical() bool {
	return r == VerticalTTB_RTL || r == VerticalTTB_LTR
}

// IsLeftToRight returns true if horizontal flow is left-to-right.
func (r ReadingOrder) IsLeftToRight() bool {
	return r == HorizontalLTR_TTB || r == VerticalTTB_LTR
}

// IsRightToLeft returns true if horizontal flow is right-to-left.
func (r ReadingOrder) IsRightToLeft() bool {
	return r == HorizontalRTL_TTB || r == VerticalTTB_RTL
}

// IsTopToBottom returns true if vertical flow is top-to-bottom.
func (r ReadingOrder) IsTopToBottom() bool {
	// All current orders are top-to-bottom
	return true
}
