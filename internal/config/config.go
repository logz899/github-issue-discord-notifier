package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"
)

var repositorySegment = regexp.MustCompile(`^[a-z0-9_.-]+$`)

type Monitor struct {
	ID         string   `json:"id"`
	Repository string   `json:"repository"`
	Labels     []string `json:"labels"`
	Channel    string   `json:"channel"`
}

type Config struct {
	Monitors []Monitor `json:"monitors"`
}

type Destinations map[string]string

func Load(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open config: %w", err)
	}
	defer file.Close()

	var cfg Config
	if err := decodeOne(file, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	return cfg, nil
}

func ParseDestinations(raw string) (Destinations, error) {
	var parsed Destinations
	if err := decodeOne(strings.NewReader(raw), &parsed); err != nil {
		return nil, errors.New("decode Discord destinations")
	}
	if len(parsed) == 0 {
		return nil, errors.New("Discord destinations must not be empty")
	}

	normalized := make(Destinations, len(parsed))
	for alias, webhookURL := range parsed {
		key := normalize(alias)
		if key == "" {
			return nil, errors.New("Discord destination alias must not be blank")
		}
		webhookURL = strings.TrimSpace(webhookURL)
		if webhookURL == "" {
			return nil, fmt.Errorf("Discord destination %q has a blank URL", key)
		}
		parsedURL, err := url.Parse(webhookURL)
		if err != nil || parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
			return nil, fmt.Errorf("Discord destination %q has an invalid URL", key)
		}
		if _, exists := normalized[key]; exists {
			return nil, fmt.Errorf("duplicate Discord destination alias %q", key)
		}
		normalized[key] = webhookURL
	}
	return normalized, nil
}

func Validate(cfg Config, destinations Destinations) (Config, error) {
	if len(cfg.Monitors) == 0 {
		return Config{}, errors.New("config must contain at least one monitor")
	}

	knownDestinations := make(map[string]struct{}, len(destinations))
	for alias := range destinations {
		key := normalize(alias)
		if key == "" {
			return Config{}, errors.New("Discord destination alias must not be blank")
		}
		if _, duplicate := knownDestinations[key]; duplicate {
			return Config{}, fmt.Errorf("duplicate Discord destination alias %q", key)
		}
		knownDestinations[key] = struct{}{}
	}

	normalized := Config{Monitors: make([]Monitor, 0, len(cfg.Monitors))}
	seenIDs := make(map[string]struct{}, len(cfg.Monitors))
	for _, monitor := range cfg.Monitors {
		monitor.ID = normalize(monitor.ID)
		if monitor.ID == "" {
			return Config{}, errors.New("monitor ID must not be blank")
		}
		if _, duplicate := seenIDs[monitor.ID]; duplicate {
			return Config{}, fmt.Errorf("duplicate monitor ID %q", monitor.ID)
		}
		seenIDs[monitor.ID] = struct{}{}

		monitor.Repository = normalize(monitor.Repository)
		parts := strings.Split(monitor.Repository, "/")
		if len(parts) != 2 || !repositorySegment.MatchString(parts[0]) || !repositorySegment.MatchString(parts[1]) {
			return Config{}, fmt.Errorf("monitor %q repository must use owner/name syntax", monitor.ID)
		}

		seenLabels := make([]string, 0, len(monitor.Labels))
		for index, label := range monitor.Labels {
			label = normalize(label)
			if label == "" {
				return Config{}, fmt.Errorf("monitor %q contains a blank label", monitor.ID)
			}
			for _, existing := range seenLabels {
				if strings.EqualFold(existing, label) {
					return Config{}, fmt.Errorf("monitor %q contains duplicate label %q", monitor.ID, label)
				}
			}
			seenLabels = append(seenLabels, label)
			monitor.Labels[index] = label
		}

		monitor.Channel = normalize(monitor.Channel)
		if monitor.Channel == "" {
			return Config{}, fmt.Errorf("monitor %q channel must not be blank", monitor.ID)
		}
		if _, exists := knownDestinations[monitor.Channel]; !exists {
			return Config{}, fmt.Errorf("monitor %q references unknown channel %q", monitor.ID, monitor.Channel)
		}

		normalized.Monitors = append(normalized.Monitors, monitor)
	}
	return normalized, nil
}

func decodeOne(reader io.Reader, target any) error {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON documents")
		}
		return err
	}
	return nil
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
