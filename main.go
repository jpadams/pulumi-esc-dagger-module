// Esc functions for pulumiConfig and opening environment for env vars (OIDC)

package main

import (
	"context"
	"dagger/esc/internal/dagger"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Esc struct{
	// +private
	EscEnv      string
	// +private
	PulumiToken *dagger.Secret
	// +private
	EscOpenEnv  string
}

func (m *Esc) container() (*dagger.Container, error) {
	if m.PulumiToken == nil {
		return nil, fmt.Errorf("need to set Pulumi token using with-token")
	}
	if m.EscEnv == "" {
		return nil, fmt.Errorf("need to set ESC environment using with-env")
	}
	return dag.Container().
		From("alpine:latest").
		WithSecretVariable("PULUMI_ACCESS_TOKEN", m.PulumiToken).
		WithExec([]string{"apk", "add", "curl", "jq"}).
		WithExec([]string{"sh", "-c", "curl -fsSL https://get.pulumi.com/esc/install.sh | sh"}), nil
}

// Set Pulumi token
func (m *Esc) WithToken(token *dagger.Secret) *Esc {
	m.PulumiToken = token
	return m
}

// Set Pulumi environment
func (m *Esc) WithEnv(env string) *Esc {
	m.EscEnv = env
	return m
}

// Get Pulumi ESC values from pulumiConfig object
func (m *Esc) GetConfig(ctx context.Context, name string) (string, error) {
	ctr, _ := m.container()
	val, err := ctr.
		WithEnvVariable("CACHEBUSTER", time.Now().String()).
		WithExec([]string{"sh", "-c", fmt.Sprintf("$HOME/.pulumi/bin/esc env get %s pulumiConfig.%s --value json | jq -r", m.EscEnv, name)}).
		Stdout(ctx)
	return strings.TrimSpace(val), err
}

// Set Pulumi ESC name/value pairs on the pulumiConfig object
func (m *Esc) SetConfig(ctx context.Context, name string, value string) error {
	ctr, _ := m.container()
	_, err := ctr.
		WithEnvVariable("CACHEBUSTER", time.Now().String()).
		WithExec([]string{"sh", "-c", fmt.Sprintf("$HOME/.pulumi/bin/esc env set %s pulumiConfig.%s %s", m.EscEnv, name, value)}).
		Stdout(ctx)
	return err
}

// Open the Pulum ESC environment to access env vars
func (m *Esc) Open(ctx context.Context) *Esc {
	ctr, _ := m.container()
	val, _ := ctr.
		WithEnvVariable("CACHEBUSTER", time.Now().String()).
		WithExec([]string{"sh", "-c", fmt.Sprintf("$HOME/.pulumi/bin/esc env open %s | jq '.environmentVariables'", m.EscEnv)}).
		Stdout(ctx)
	envJson := strings.TrimSpace(val)
	m.EscOpenEnv = envJson
	return m
}

// Get env vars from opened Pulumi ESC environment as Secrets
func (m *Esc) GetSecretEnvVar(ctx context.Context, name string) (*dagger.Secret, error) {
	if m.EscOpenEnv == "" {
		return nil, fmt.Errorf("need to open environment using open")
	}
	val, err := LookupKey(m.EscOpenEnv, name)
	if err != nil {
		return nil, fmt.Errorf("requested field %v is empty or not a string: %s", name, err)
	}
	return dag.SetSecret(name, val), nil
}


// Helper: Looks up a key in the given JSON string and returns the value as a string without surrounding quotes.
func LookupKey(jsonStr, key string) (string, error) {
	var data map[string]interface{}

	err := json.Unmarshal([]byte(jsonStr), &data)
	if err != nil {
		return "", fmt.Errorf("failed to parse JSON: %w", err)
	}

	value, ok := data[key]
	if !ok {
		return "", errors.New("key not found in JSON")
	}

	// Convert the value to a string without surrounding quotes
	switch v := value.(type) {
	case string:
		return v, nil
	case float64:
		return fmt.Sprintf("%g", v), nil
	case bool:
		return fmt.Sprintf("%t", v), nil
	default:
		// For complex types like arrays or objects, return as a JSON string
		jsonValue, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("failed to convert value to string: %w", err)
		}
		return string(jsonValue), nil
	}
}
