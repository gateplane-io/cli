// Copyright (C) 2026 Ioannis Torakis <john.torakis@gmail.com>
// SPDX-License-Identifier: Elastic-2.0
//
// Licensed under the Elastic License 2.0.
// You may obtain a copy of the license at:
// https://www.elastic.co/licensing/elastic-license
//
// Use, modification, and redistribution permitted under the terms of the license,
// except for providing this software as a commercial service or product.

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
	Description string   `json:"description" yaml:"description,omitempty"`
}

// EntityAlias represents a Vault entity alias
type EntityAlias struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	CanonicalMount string `json:"canonical_mount"`
	MountAccessor  string `json:"mount_accessor"`
	MountPath      string `json:"mount_path"`
	MountType      string `json:"mount_type"`
}

// Entity represents a Vault entity with its aliases
type Entity struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Aliases  []EntityAlias          `json:"aliases"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Policies []string               `json:"policies,omitempty"`
}

// Self contains both the entity and alias information for the calling user
// Using standard Vault API response structures from LookupSelf()
type Self struct {
	*Entity      `json:"entity,omitempty"`
	*EntityAlias `json:"alias,omitempty"`
	// Additional information from the token response
	DisplayName string `json:"display_name"`
	Username    string `json:"username"`
}
