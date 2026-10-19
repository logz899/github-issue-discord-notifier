package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const currentVersion = 1

type Monitor struct {
	LastSuccessfulPoll time.Time `json:"last_successful_poll"`
	NotifiedIssueIDs   []int64   `json:"notified_issue_ids"`
}

type State struct {
	Version  int                `json:"version"`
	Monitors map[string]Monitor `json:"monitors"`
}

func Load(path string) (State, error) {
	file, err := os.Open(path)
	if err != nil {
		return State{}, fmt.Errorf("open state: %w", err)
	}
	defer file.Close()

	var value State
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return State{}, fmt.Errorf("decode state: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return State{}, errors.New("decode state: multiple JSON documents")
		}
		return State{}, fmt.Errorf("decode state trailing data: %w", err)
	}
	if err := validate(value); err != nil {
		return State{}, err
	}
	return value, nil
}

func Save(path string, value State) error {
	if err := validate(value); err != nil {
		return err
	}

	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".state-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary state: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		temporary.Close()
		return fmt.Errorf("encode temporary state: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync temporary state: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary state: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace state: %w", err)
	}
	return nil
}

func validate(value State) error {
	if value.Version != currentVersion {
		return fmt.Errorf("unsupported state version %d", value.Version)
	}
	if value.Monitors == nil {
		return errors.New("state monitors must be an object")
	}
	for routeID, monitor := range value.Monitors {
		if strings.TrimSpace(routeID) == "" {
			return errors.New("state monitor ID must not be blank")
		}
		if monitor.LastSuccessfulPoll.IsZero() {
			return fmt.Errorf("state monitor %q has no successful poll timestamp", routeID)
		}
		seen := make(map[int64]struct{}, len(monitor.NotifiedIssueIDs))
		for _, issueID := range monitor.NotifiedIssueIDs {
			if issueID <= 0 {
				return fmt.Errorf("state monitor %q has invalid issue ID", routeID)
			}
			if _, duplicate := seen[issueID]; duplicate {
				return fmt.Errorf("state monitor %q has duplicate issue ID %d", routeID, issueID)
			}
			seen[issueID] = struct{}{}
		}
	}
	return nil
}
