package agent_adapter

import (
	"fmt"
	"sort"
	"time"
)

type AgentType string

const (
	AgentTypeClaudeCode AgentType = "claude-code"
	AgentTypeCodex      AgentType = "codex"
	AgentTypeOpenCode   AgentType = "opencode"
)

type BaseOptions struct {
	Home       string
	GatewayURL string
	Token      string
	Model      string
	Now        func() time.Time
}

type BaseRestoreOptions struct {
	Home string
}

type AgentConfig interface {
	Type() AgentType
	Write(BaseOptions) (Result, error)
	Restore(BaseRestoreOptions) (Result, error)
}

type Registry struct {
	configs map[AgentType]AgentConfig
}

var DefaultRegistry = NewRegistry()

func init() {
	DefaultRegistry.MustRegister(&ClaudeCodeConfig{})
	DefaultRegistry.MustRegister(&CodexConfig{})
	DefaultRegistry.MustRegister(&OpenCodeConfig{})
}

func NewRegistry() *Registry {
	return &Registry{configs: map[AgentType]AgentConfig{}}
}

func (r *Registry) Register(config AgentConfig) error {
	if config == nil {
		return fmt.Errorf("agent config is nil")
	}
	agentType := config.Type()
	if agentType == "" {
		return fmt.Errorf("agent type is empty")
	}
	if _, exists := r.configs[agentType]; exists {
		return fmt.Errorf("agent config %q already registered", agentType)
	}
	r.configs[agentType] = config
	return nil
}

func (r *Registry) MustRegister(config AgentConfig) {
	if err := r.Register(config); err != nil {
		panic(err)
	}
}

func (r *Registry) Get(agentType AgentType) (AgentConfig, bool) {
	config, ok := r.configs[agentType]
	return config, ok
}

func (r *Registry) List() []AgentType {
	types := make([]AgentType, 0, len(r.configs))
	for agentType := range r.configs {
		types = append(types, agentType)
	}
	sort.Slice(types, func(i, j int) bool {
		return types[i] < types[j]
	})
	return types
}
