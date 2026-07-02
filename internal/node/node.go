package node

// Node represents a target host for execution, corresponding to Rundeck's INodeEntry.
type Node struct {
	Name        string            `json:"name"`
	Hostname    string            `json:"hostname"`
	Username    string            `json:"username"`
	OSName      string            `json:"osName,omitempty"`
	OSFamily    string            `json:"osFamily,omitempty"`
	OSVersion   string            `json:"osVersion,omitempty"`
	OSArch      string            `json:"osArch,omitempty"`
	Description string            `json:"description,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
}

// Registry manages project nodes.
type Registry interface {
	GetNode(name string) (*Node, error)
	ListNodes() ([]*Node, error)
}
