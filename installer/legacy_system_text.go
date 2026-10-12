package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type legacySystemText struct {
	Data     []byte         `json:"data"`
	Mode     uint32         `json:"mode"`
	Snapshot configSnapshot `json:"snapshot"`
}

func (file legacySystemText) validate() error {
	if len(file.Data) > maxProfileBytes || file.Mode > 0777 {
		return errors.New("system text exceeds its inspection bound")
	}
	if file.Snapshot.Kind == "absent" && file.Data == nil && file.Mode == 0 && file.Snapshot.Hash == "" {
		return nil
	}
	expected, err := snapshotEntries([]configEntry{{Path: ".", Kind: "file", Content: legacyHash(file.Data), Mode: file.Mode}})
	if err != nil || file.Snapshot != expected {
		return errors.Join(errors.New("system text does not match its inspected snapshot"), err)
	}
	return nil
}

// InspectLegacySystemText is a bounded read-only entrypoint for the vendor's
// root-private configuration. Links and directories are never followed. The
// bytes stay in the controller; they are not included in diagnostics.
func InspectLegacySystemText(path string, output io.Writer) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || len(path) > 4096 || strings.ContainsRune(path, 0) {
		return errors.New("system text inspection requires a bounded absolute path")
	}
	data, snapshot, err := readLegacyFile(path, maxProfileBytes)
	if err != nil {
		return err
	}
	file := legacySystemText{Data: data, Snapshot: snapshot}
	if snapshot.Kind != "absent" {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		file.Mode = uint32(info.Mode().Perm())
	}
	if err := file.validate(); err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(file)
}

func (d *legacyRetirementDriver) readSystemText(ctx context.Context, path string) ([]byte, configSnapshot, error) {
	data, snapshot, err := readLegacyFile(path, maxProfileBytes)
	if !errors.Is(err, os.ErrPermission) {
		return data, snapshot, err
	}
	output, err := d.privilegedQuery(ctx, d.systemCommand(true, d.Environment.Executable, "internal-legacy-system-text", path))
	if err != nil {
		return nil, configSnapshot{}, fmt.Errorf("read-only administrator inspection failed; refresh credentials with sudo -v before retrying migration: %w", err)
	}
	var file legacySystemText
	decoder := json.NewDecoder(bytes.NewReader(output))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		return nil, configSnapshot{}, errors.New("invalid privileged system text response")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, configSnapshot{}, errors.New("privileged system text response has trailing data")
	}
	if err := file.validate(); err != nil {
		return nil, configSnapshot{}, err
	}
	return file.Data, file.Snapshot, nil
}

func (d *legacyRetirementDriver) verifyLegacySnapshot(ctx context.Context, file legacyRetirementFile, privileged bool) error {
	err := verifyConfigSnapshot(file.Path, file.Snapshot)
	if !privileged || !errors.Is(err, os.ErrPermission) {
		return err
	}
	_, current, err := d.readSystemText(ctx, file.Path)
	if err != nil || current != file.Snapshot {
		return errors.Join(fmt.Errorf("configuration artifact changed at %s; preserve it for recovery", file.Path), err)
	}
	return nil
}
