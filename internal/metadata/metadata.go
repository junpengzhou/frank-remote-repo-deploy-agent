package metadata

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	Filename         = "salt-agent-metadata.json"
	SchemaVersion    = "1.0"
	RoleMain         = "main"
	RoleDependency   = "dependency"
	descriptionLimit = 100
)

type Document struct {
	SchemaVersion string   `json:"schemaVersion"`
	GeneratedAt   string   `json:"generatedAt"`
	Environment   string   `json:"environment"`
	Branch        string   `json:"branch"`
	MainModule    string   `json:"mainModule"`
	Build         Build    `json:"build"`
	Modules       []Module `json:"modules"`
}

type Build struct {
	StartedAt    string `json:"startedAt"`
	FinishedAt   string `json:"finishedAt"`
	DurationMs   int64  `json:"durationMs"`
	BuildSkipped bool   `json:"buildSkipped"`
}

type Module struct {
	Name    string   `json:"name"`
	Role    string   `json:"role"`
	Commits []Commit `json:"commits"`
}

type Commit struct {
	Hash        string    `json:"hash"`
	Committer   Committer `json:"committer"`
	CommittedAt string    `json:"committedAt"`
	Description string    `json:"description"`
}

type Committer struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func TruncateDescription(value string) string {
	runes := []rune(value)
	if len(runes) <= descriptionLimit {
		return value
	}
	return string(runes[:descriptionLimit]) + "..."
}

func Write(stagingDir string, doc Document) error {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	temp, err := os.CreateTemp(stagingDir, ".salt-agent-metadata-*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer func() {
		_ = os.Remove(tempName)
	}()

	if err := temp.Chmod(0o644); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, filepath.Join(stagingDir, Filename))
}
