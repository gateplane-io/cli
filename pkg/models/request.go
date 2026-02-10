package models

import (
	"github.com/gateplane-io/vault-plugins/pkg/responses"
)

type Request struct {
	*responses.AccessRequestResponse
	*Gate
}
