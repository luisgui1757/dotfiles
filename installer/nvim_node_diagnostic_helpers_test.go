package installer

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// This temporary failure-only fixture diagnosis never runs Node/npm and never
// changes an installed generation. Reconstruct only a proved payload mismatch.
func nativeNodeArchiveDiagnostic(ctx context.Context, d *ArchiveDriver, reference string) (string, error) {
	const id = "tool.node"
	var output strings.Builder
	o, err := d.Observe(ctx, Resource{ID: id}, Receipt{})
	fmt.Fprintf(&output, "observation present=%v healthy=%v unknown=%v desired=%s fingerprint=%s pending=%q\n", o.Present, o.Healthy, o.Unknown, o.Desired, o.Fingerprint, o.Pending)
	if err != nil {
		return output.String(), err
	}
	current, err := d.current(id)
	if err != nil || current.Version == "" {
		return output.String(), errors.Join(errors.New("Node has no readable current generation"), err)
	}
	version, err := d.version(id, current.Version)
	if err != nil {
		return output.String(), err
	}
	payload := filepath.Join(d.versionDirectory(id, current.Version), "payload")
	link, err := archiveLinkTarget(d.currentLink(id))
	fmt.Fprintf(&output, "pin desired=%s saved=%s; current=%s operation=%s\nlink actual=%q expected=%q\n", archivePinID(d.Pins[id]), archivePinID(version.Pin), current.Version, current.Operation, link, payload)
	if err != nil {
		return output.String(), err
	}
	actual, err := inspectTree(payload, maxPackageBytes, maxPackageEntries)
	if err != nil {
		return output.String(), err
	}
	snapshot, err := snapshotEntries(actual)
	fmt.Fprintf(&output, "payload actual=%+v saved=%+v\n", snapshot, version.Payload)
	if err != nil || snapshot == version.Payload {
		return output.String(), err
	}
	pin := version.Pin
	if pin.PythonStdlib != "" || len(pin.RustComponents) != 0 || pin.Latex2text != nil || pin.Yamllint != nil || pin.PortableGit || pin.GhosttyLibraries {
		return output.String(), errors.New("Node diagnostic requires its plain pinned archive recipe")
	}
	// macOS temporary paths may use /var while symlink resolution uses
	// /private/var. Compare archive links within the same physical parent.
	parent, err := resolveConfigPath(filepath.Dir(reference))
	if err != nil {
		return output.String(), err
	}
	reference = filepath.Join(parent, filepath.Base(reference))
	if err := downloadArchive(ctx, d.Client, pin, reference); err != nil {
		return output.String(), err
	}
	expected, err := inspectTree(reference, maxPackageBytes, maxPackageEntries)
	if err != nil {
		return output.String(), err
	}
	baseline, err := snapshotEntries(expected)
	if err != nil || baseline != version.Payload {
		return output.String(), errors.Join(fmt.Errorf("reconstructed reference %v differs from saved publication %v; no authoritative entry diff", baseline, version.Payload), err)
	}
	output.WriteString(nativeNodeEntryDiff(expected, actual))
	return output.String(), nil
}

func nativeNodeEntryDiff(expected, actual []configEntry) string {
	want, got := map[string]configEntry{}, map[string]configEntry{}
	paths := []string{}
	for _, entry := range expected {
		want[entry.Path] = entry
		paths = append(paths, entry.Path)
	}
	for _, entry := range actual {
		got[entry.Path] = entry
		if _, ok := want[entry.Path]; !ok {
			paths = append(paths, entry.Path)
		}
	}
	slices.Sort(paths)
	var output strings.Builder
	count := 0
	for _, path := range paths {
		if want[path] == got[path] {
			continue
		}
		count++
		if count > 40 {
			continue
		}
		// File Content is already a SHA-256; link targets and names are clipped.
		clip := func(s string) string {
			if len(s) > 256 {
				return s[:256] + "[truncated]"
			}
			return s
		}
		w, g := want[path], got[path]
		fmt.Fprintf(&output, "%q: expected kind=%q mode=%04o content=%q; actual kind=%q mode=%04o content=%q\n", clip(path), w.Kind, w.Mode, clip(w.Content), g.Kind, g.Mode, clip(g.Content))
	}
	fmt.Fprintf(&output, "Node payload entry differences=%d (at most 40 shown)\n", count)
	return output.String()
}
