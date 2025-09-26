package models

// GateType represents the type of a gate
type GateType string

// Gate types supported by GatePlane
const (
	PolicyGate    GateType = "policy-gate"
	OktaGroupGate GateType = "okta-group-gate"
)

// Gate represents a GatePlane gate configuration
type Gate struct {
	Path        string   `json:"path" yaml:"path"`
	Type        GateType `json:"type" yaml:"type"`
	Alias       string   `json:"alias,omitempty" yaml:"alias,omitempty"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
}
