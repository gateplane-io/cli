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

import (
	"github.com/gateplane-io/vault-plugins/pkg/responses"
)

// RequestServiceResponse represents the complete request service response
type RequestServiceResponse struct {
	Request *responses.AccessRequestResponse `json:"request"`
	Gate    Gate                             `json:"gate"`
	Access  []Access                         `json:"access"`
}
