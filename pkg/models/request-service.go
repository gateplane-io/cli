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
