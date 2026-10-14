package docker

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/JeremyOrr4/caged/internal/engine"
)

// parseInspect reads the JSON array `docker image inspect` prints and returns
// the facts about its first image.
func parseInspect(data []byte) (engine.ImageInfo, error) {
	var replies []struct {
		ID           string `json:"Id"`
		OS           string `json:"Os"`
		Architecture string
		Config       struct{ Labels map[string]string }
	}
	if err := json.Unmarshal(data, &replies); err != nil {
		return engine.ImageInfo{}, fmt.Errorf("parse docker image inspect output: %w", err)
	}
	if len(replies) == 0 {
		return engine.ImageInfo{}, errors.New("parse docker image inspect output: no image in the reply")
	}
	reply := replies[0]
	return engine.ImageInfo{ID: reply.ID, OS: reply.OS, Arch: reply.Architecture, Labels: reply.Config.Labels}, nil
}

// parseImageList reads the output of `docker image ls --format '{{json .}}'`:
// one JSON object per line. A "Labels" field, when present, is a string of
// comma-separated key=value pairs.
func parseImageList(data []byte) ([]engine.ImageSummary, error) {
	var summaries []engine.ImageSummary
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var row struct {
			ID, Repository, Tag, Size, Labels string
		}
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("parse docker image ls output: %w", err)
		}
		summaries = append(summaries, engine.ImageSummary{ID: row.ID, Repository: row.Repository, Tag: row.Tag, Size: row.Size, Labels: parseLabelList(row.Labels)})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parse docker image ls output: %w", err)
	}
	return summaries, nil
}

// parseLabelList parses "k=v,k=v" into a map; an empty string gives nil.
func parseLabelList(list string) map[string]string {
	if list == "" {
		return nil
	}
	labels := map[string]string{}
	for _, pair := range strings.Split(list, ",") {
		key, value, _ := strings.Cut(pair, "=")
		labels[key] = value
	}
	return labels
}
