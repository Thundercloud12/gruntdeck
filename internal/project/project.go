package project

import "github.com/Thundercloud12/gruntdeck/internal/node"

// Project represents a workspace container for jobs and nodes.
type Project struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Config      map[string]string `json:"config,omitempty"`
}

// Manager handles the lifecycle of projects.
type Manager interface {
	CreateProject(p *Project) error
	GetProject(name string) (*Project, error)
	ListProjects() ([]*Project, error)
	DeleteProject(name string) error
	GetNodeRegistry(projectName string) (node.Registry, error)
}
