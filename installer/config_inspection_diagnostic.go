package installer

import (
	"errors"
	"fmt"
	"os"
)

// Keep the existing refusal; only the failed entry's relative name and metadata
// are diagnostic. A later native identity probe does not certify the old scan.
func configInspectionFailure(path, relative, kind string, before, after os.FileInfo, same bool, statErr error) error {
	if len(relative) > 512 {
		relative = relative[:512] + "[truncated]"
	}
	metadata := func(info os.FileInfo) string {
		if info == nil {
			return "unavailable"
		}
		return fmt.Sprintf("mode=%s size=%d mtime_ns=%d", info.Mode(), info.Size(), info.ModTime().UnixNano())
	}
	identity := "not needed"
	if statErr == nil && !same {
		identity = configInspectionIdentityDiagnostic(path)
	}
	return errors.Join(fmt.Errorf("configuration changed during inspection; review a fresh plan: entry=%q kind=%s same_file=%v before={%s} after={%s} subsequent_identity_probe={%s}", relative, kind, same, metadata(before), metadata(after), identity), statErr)
}
