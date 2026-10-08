package installer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

const nvimManifestChunkEntries = 4096

type nvimSyncManifestChunk struct {
	Schema    int           `json:"schema"`
	Operation string        `json:"operation"`
	Index     int           `json:"index"`
	Entries   []configEntry `json:"entries"`
}

func (d *NvimSyncDriver) manifestPath(operation string, index int) string {
	return filepath.Join(d.Directory, "manifests", operation, fmt.Sprintf("%03d.json", index))
}

// Native plugin, grammar and language-tool trees exceed a single document.
// Fixed chunks preserve the shared document limit and existing cleanup format.
func (d *NvimSyncDriver) writeManifest(operation string, entries []configEntry) ([]string, error) {
	if err := validateCleanupEntries(entries, maxNvimEntries); err != nil {
		return nil, err
	}
	hashes := []string{}
	for begin := 0; begin < len(entries); begin += nvimManifestChunkEntries {
		end := min(begin+nvimManifestChunkEntries, len(entries))
		chunk := nvimSyncManifestChunk{Schema: 1, Operation: operation, Index: len(hashes), Entries: entries[begin:end]}
		hash, err := digest(chunk)
		if err != nil {
			return nil, err
		}
		path := d.manifestPath(operation, chunk.Index)
		data, err := readDocument(path)
		if err == nil {
			var old nvimSyncManifestChunk
			if err := Decode(data, &old); err != nil {
				return nil, err
			}
			previous, err := digest(old)
			if err != nil || previous != hash {
				return nil, errors.Join(errors.New("Neovim manifest chunk changed; preserve the staged runtime"), err)
			}
		} else if errors.Is(err, os.ErrNotExist) {
			if err := saveDocument(path, chunk); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
		hashes = append(hashes, hash)
	}
	return hashes, nil
}

func (d *NvimSyncDriver) readManifest(intent nvimSyncIntent) ([]configEntry, error) {
	if len(intent.Manifest) == 0 || len(intent.Manifest) > (maxNvimEntries+nvimManifestChunkEntries-1)/nvimManifestChunkEntries {
		return nil, errors.New("invalid Neovim manifest count")
	}
	entries := []configEntry{}
	for index, want := range intent.Manifest {
		if !operationID.MatchString(want) {
			return nil, errors.New("invalid Neovim manifest digest")
		}
		data, err := readDocument(d.manifestPath(intent.Operation, index))
		if err != nil {
			// A missing chunk is corrupt ownership, not a missing operation.
			return nil, fmt.Errorf("cannot read Neovim manifest chunk %d: %v", index, err)
		}
		var chunk nvimSyncManifestChunk
		if err := Decode(data, &chunk); err != nil {
			return nil, err
		}
		hash, err := digest(chunk)
		if err != nil || hash != want || chunk.Schema != 1 || chunk.Operation != intent.Operation || chunk.Index != index || len(chunk.Entries) == 0 || len(chunk.Entries) > nvimManifestChunkEntries || index < len(intent.Manifest)-1 && len(chunk.Entries) != nvimManifestChunkEntries {
			return nil, errors.Join(errors.New("Neovim manifest chunk differs from saved ownership"), err)
		}
		entries = append(entries, slices.Clone(chunk.Entries)...)
	}
	if err := validateCleanupEntries(entries, maxNvimEntries); err != nil {
		return nil, err
	}
	return entries, nil
}
