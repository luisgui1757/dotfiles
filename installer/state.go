package installer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

var operationID = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Transaction struct {
	Plan     Plan              `json:"plan"`
	Next     int               `json:"next"`
	InFlight string            `json:"in_flight,omitempty"`
	Error    string            `json:"error,omitempty"`
	Pending  map[string]string `json:"pending,omitempty"`
}

type State struct {
	Schema           int                `json:"schema"`
	Context          Context            `json:"context"`
	Target           string             `json:"target"`
	Source           string             `json:"source"`
	Generation       uint64             `json:"generation"`
	Selected         []string           `json:"selected"`
	Keep             []string           `json:"keep"`
	Receipts         map[string]Receipt `json:"receipts"`
	Transaction      *Transaction       `json:"transaction,omitempty"`
	PastTransactions []string           `json:"past_transactions,omitempty"`
}

func Decode(data []byte, target any) error {
	if len(data) > 8*1024*1024 {
		return fmt.Errorf("installer document exceeds 8 MiB")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("unexpected content after installer document")
	}
	return nil
}

func LoadState(path, target string) (State, error) {
	state := State{Target: target, Receipts: map[string]Receipt{}}
	data, err := readDocument(path)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	var header struct {
		Schema int `json:"schema"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return state, fmt.Errorf("invalid state; preserve it for recovery: %w", err)
	}
	if header.Schema != 1 {
		return state, fmt.Errorf("unsupported state schema %d; use the matching installer", header.Schema)
	}
	if err := Decode(data, &state); err != nil {
		return state, fmt.Errorf("invalid state; preserve it for recovery: %w", err)
	}
	if state.Schema != 1 {
		return state, fmt.Errorf("unsupported state schema %d; use the matching installer", state.Schema)
	}
	if state.Target != target {
		return state, fmt.Errorf("state target differs from the active user home")
	}
	if state.Receipts == nil {
		state.Receipts = map[string]Receipt{}
	}
	for id, receipt := range state.Receipts {
		if !resourceID.MatchString(id) {
			return state, fmt.Errorf("invalid receipt ID %q", id)
		}
		if receipt.Ownership != "created" && receipt.Ownership != "reused" && receipt.Ownership != "uncertain" {
			return state, fmt.Errorf("invalid ownership for %s", id)
		}
		if receipt.Recovery != "" && (!filepath.IsLocal(receipt.Recovery) || filepath.Clean(receipt.Recovery) != receipt.Recovery) {
			return state, fmt.Errorf("invalid recovery reference for %s", id)
		}
		if receipt.OperationID != "" && !operationID.MatchString(receipt.OperationID) {
			return state, fmt.Errorf("invalid operation identity for %s", id)
		}
		if receipt.Adopted && !receipt.Before.Present {
			return state, fmt.Errorf("adopted resource %s has no original baseline", id)
		}
	}
	for _, archive := range state.PastTransactions {
		if !filepath.IsLocal(archive) || filepath.Dir(archive) != "transactions" || len(filepath.Base(archive)) != 69 || filepath.Ext(archive) != ".json" {
			return state, errors.New("invalid transaction archive reference")
		}
	}
	return state, nil
}

// OpenRoot uses delete-sharing on Windows, allowing atomic publication while
// a preview reads the previous version. Reads remain bounded and beneath the
// directory; a changed inode or symlink is rejected before parsing.
func readDocument(path string) (data []byte, result error) {
	directory := filepath.Dir(path)
	info, err := os.Lstat(directory)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("installer document directory must not be a link")
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, root.Close()) }()
	name := filepath.Base(path)
	before, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Size() > 8*1024*1024 {
		return nil, errors.New("installer document must be a regular file of at most 8 MiB")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, file.Close()) }()
	after, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(before, after) {
		return nil, errors.New("installer document changed while opening; retry discovery")
	}
	data, err = io.ReadAll(io.LimitReader(file, 8*1024*1024+1))
	if len(data) > 8*1024*1024 {
		return nil, errors.New("installer document exceeds 8 MiB")
	}
	return data, err
}

// SaveState publishes only complete JSON. The old state survives a failed write.
func SaveState(path string, state State) (result error) {
	return saveDocument(path, state)
}

func saveDocument(path string, value any) (result error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if len(data) > 8*1024*1024 {
		return errors.New("installer document exceeds 8 MiB; existing state preserved")
	}
	dir := filepath.Dir(path)
	if err := prepareStateDirectory(dir); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("state destination must be a regular file")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	file, err := os.CreateTemp(dir, ".state-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer func() {
		if err := os.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
			result = errors.Join(result, err)
		}
	}()
	if err := file.Chmod(0600); err != nil {
		return errors.Join(err, file.Close())
	}
	if _, err := file.Write(data); err != nil {
		return errors.Join(err, file.Close())
	}
	if err := file.Sync(); err != nil {
		return errors.Join(err, file.Close())
	}
	if err := file.Close(); err != nil {
		return err
	}
	return publishState(name, path)
}

func archiveTransaction(directory string, transaction Transaction) (string, error) {
	id, err := digest(transaction)
	if err != nil {
		return "", err
	}
	relative := filepath.Join("transactions", id+".json")
	path := filepath.Join(directory, relative)
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return "", errors.New("transaction archive must be a regular file")
		}
		data, err := readDocument(path)
		if err != nil {
			return "", err
		}
		var previous Transaction
		if err := Decode(data, &previous); err != nil {
			return "", err
		}
		previousID, err := digest(previous)
		if err != nil || previousID != id {
			return "", errors.New("transaction archive changed; preserve it for recovery")
		}
		return relative, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	return relative, saveDocument(path, transaction)
}

// The ledger directory itself must be a real directory. Higher-level known
// folders may intentionally be redirected; callers bind their resolved path
// to the target identity before planning.
func prepareStateDirectory(directory string) error {
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(directory)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("installer state directory must not be a link")
	}
	return nil
}
