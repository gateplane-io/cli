package models

// Wrap in a struct for hclsimple
type Policy struct {
	Paths []PathBlock `hcl:"path,block"`
}

// PolicyACL represents a Vault ACL policy
type PolicyACL struct {
	Name   string `json:"name"`
	Rules  string `json:"rules"` // Original HCL rules
	Parsed Policy `json:"parsed"`
}

type PathBlock struct {
	Path         string   `hcl:"path,label" json:"path"`           // the label in path "secret/*"
	Capabilities []string `hcl:"capabilities" json:"capabilities"` // the capabilities array
}

// Used to send to GatePlane Services
type AccessBlock struct {
	PathBlock   []PathBlock `json:"paths"`
	Description string      `json:"description"`
}

type Access struct {
	Policy string                 `json:"policy"`
	Types  map[string]AccessBlock `json:"accessTypes"`
}
