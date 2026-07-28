package board

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempYAML(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "switchboard.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadFileMinimal(t *testing.T) {
	body := `
server:
  listen: ":9000"
pipeline:
  length: {model_path: "./models/length.bin", default: short}
  complexity: {model_path: "./models/complexity.bin"}
  style: {model_path: "./models/style.bin"}
  quality: {model_path: "./models/quality.bin"}
  camera: {model_path: "./models/camera.bin"}
  physics: {model_path: "./models/physics.bin"}
  refs: {model_path: "./models/refs.bin"}
  cost: {model_path: "./models/cost.bin"}
providers:
  - name: runway
    base_url: "https://api.example.com/v1"
    auth: {type: bearer, env: TEST_KEY}
    models:
      - name: gen-3
        cost_per_sec: 0.05
        capabilities: [photorealistic]
`
	path := writeTempYAML(t, body)
	c, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if c.Server.Listen != ":9000" {
		t.Errorf("listen = %q", c.Server.Listen)
	}
	if c.Pipeline.Length.ModelPath != "./models/length.bin" {
		t.Errorf("length model path = %q", c.Pipeline.Length.ModelPath)
	}
	if c.Pipeline.Length.DefaultVal != "short" {
		t.Errorf("length default = %q", c.Pipeline.Length.DefaultVal)
	}
	if len(c.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(c.Providers))
	}
	if c.Providers[0].Models[0].CostPerSec != 0.05 {
		t.Errorf("cost_per_sec = %f", c.Providers[0].Models[0].CostPerSec)
	}
}

func TestResolveSecretsMissingEnv(t *testing.T) {
	c := Config{Providers: []Provider{
		{Name: "foo", Auth: Auth{Type: "bearer", Env: "BOARD_TEST_NOT_SET"}},
	}}
	secrets := c.ResolveSecrets()
	if len(secrets) != 0 {
		t.Errorf("expected 0 secrets, got %d", len(secrets))
	}
}

func TestResolveSecretsOK(t *testing.T) {
	t.Setenv("BOARD_TEST_KEY", "secret123")
	c := Config{Providers: []Provider{
		{Name: "foo", Auth: Auth{Type: "bearer", Env: "BOARD_TEST_KEY"}},
	}}
	secrets := c.ResolveSecrets()
	if secrets["foo"] != "secret123" {
		t.Errorf("secrets[foo] = %q", secrets["foo"])
	}
}

func TestResolveSecretsSkipsNone(t *testing.T) {
	c := Config{Providers: []Provider{
		{Name: "foo", Auth: Auth{Type: "none"}},
	}}
	secrets := c.ResolveSecrets()
	if len(secrets) != 0 {
		t.Errorf("expected 0 secrets, got %d", len(secrets))
	}
}

func TestProviderByName(t *testing.T) {
	c := Config{Providers: []Provider{
		{Name: "a", BaseURL: "https://a"},
		{Name: "b", BaseURL: "https://b"},
	}}
	if got := c.ProviderByName("a"); got == nil || got.Name != "a" {
		t.Errorf("ProviderByName(a) = %v", got)
	}
	if got := c.ProviderByName("nope"); got != nil {
		t.Errorf("ProviderByName(nope) = %v, want nil", got)
	}
}

func TestModelByName(t *testing.T) {
	p := Provider{Name: "test", Models: []Model{
		{Name: "m1", CostPerSec: 0.05},
		{Name: "m2", CostPerSec: 0.10},
	}}
	if got := p.ModelByName("m1"); got == nil || got.CostPerSec != 0.05 {
		t.Errorf("ModelByName(m1) = %v", got)
	}
	if got := p.ModelByName("nope"); got != nil {
		t.Errorf("ModelByName(nope) = %v, want nil", got)
	}
}

func TestModelEstCost(t *testing.T) {
	m := Model{CostPerSec: 0.05, CostPerReq: 0.01}
	cost := m.EstCost(10)
	if cost != 0.51 {
		t.Errorf("EstCost(10) = %f, want 0.51", cost)
	}
	cost = m.EstCost(0)
	if cost != 0.01 {
		t.Errorf("EstCost(0) = %f, want 0.01", cost)
	}
}
