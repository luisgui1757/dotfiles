package installer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// Only this reviewed app recipe is connected in production. Version and bundle
// bytes are deliberately not ownership identities: vendor updates replace them.
type brewCaskSpec struct {
	Application, UserApplication, Caskroom, Binary string
	Bundle, Requirement                            string
}

func isHomebrewPackage(provider string) bool {
	return provider == "homebrew-formula" || provider == "homebrew-cask"
}

func zenCaskSpec(home, prefix string) brewCaskSpec {
	return brewCaskSpec{
		Application: "/Applications/Zen.app", UserApplication: filepath.Join(home, "Applications", "Zen.app"),
		Caskroom: filepath.Join(prefix, "Caskroom", "zen"), Binary: filepath.Join(prefix, "bin", "zen"),
		Bundle:      "app.zen-browser.zen",
		Requirement: `anchor apple generic and identifier "app.zen-browser.zen" and certificate 1[field.1.2.840.113635.100.6.2.6] exists and certificate leaf[field.1.2.840.113635.100.6.1.13] exists and certificate leaf[subject.OU] = "9V5K9TP787"`,
	}
}

func (s brewCaskSpec) validate(token string) error {
	if !brewPackageName.MatchString(token) || s.Bundle == "" || s.Requirement == "" {
		return errors.New("invalid native application recipe")
	}
	for _, path := range []string{s.Application, s.UserApplication, s.Caskroom, s.Binary} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.ContainsAny(path, "\x00\r\n") {
			return errors.New("invalid native application path")
		}
	}
	return nil
}

func (s brewCaskSpec) arguments(action, token string) []string {
	switch action {
	case "install", "repair":
		verb := "install"
		if action == "repair" {
			verb = "reinstall"
		}
		// The browser is a GUI app. Avoid claiming a shared command wrapper;
		// uninstall still needs the explicit binary collision guard below.
		return []string{verb, "--cask", "--appdir=" + filepath.Dir(s.Application), "--no-binaries", "--require-sha", token}
	case "remove":
		return []string{"uninstall", "--cask", token}
	}
	return nil
}

func (d *BrewDriver) commandEnvironment(resource, operation string, schema int) []string {
	env := slices.Clone(d.Environment)
	if _, cask := d.Casks[resource]; cask {
		env = append(env, "HOMEBREW_CASK_OPTS=", "HOMEBREW_NO_SUDO=1")
	}
	if schema >= 4 {
		env = append(env, "HOMEBREW_NO_EMOJI=")
	}
	return append(env, "HOMEBREW_INSTALL_BADGE=dotfiles-operation-"+operation)
}

func brewRemovedNames(command nativeCommand) []string {
	if len(command.Arguments) < 3 || command.Arguments[0] != "uninstall" {
		return nil
	}
	if command.Arguments[1] == "--cask" {
		return []string{"cask/" + command.Arguments[2]}
	}
	return command.Arguments[3:]
}

func (d *BrewDriver) caskIdentity(ctx context.Context, s brewCaskSpec, path string) (present, healthy bool, err error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	if !info.IsDir() {
		return true, false, nil
	}
	bound, err := bindConfigDestination(path)
	if err != nil || bound != path {
		return true, false, errors.Join(errors.New("application parent was redirected"), err)
	}
	// Apple's codesign(1) separates sealed-symlink validation from optional
	// sideband rejection. Official DMGs may retain Finder metadata; it is not
	// executable identity. Keep all signature/resource and symlink checks without
	// rewriting attributes, clearing quarantine or re-signing the vendor bundle.
	_, err = d.Query(ctx, false, "/usr/bin/codesign", nil, "--verify", "--deep", "--strict=symlinks", "-R", "="+s.Requirement, path)
	if err == nil {
		return true, true, nil
	}
	if ctx.Err() != nil {
		return true, false, ctx.Err()
	}
	var status *exec.ExitError
	if errors.As(err, &status) && (status.ExitCode() == 1 || status.ExitCode() == 3) {
		return true, false, nil
	}
	return true, false, fmt.Errorf("application identity could not be inspected: %w", err)
}

func (d *BrewDriver) observeCask(ctx context.Context, r Resource, o Observation, pkg nativePackage, s brewCaskSpec, owned bool) (Observation, error) {
	o.Provider, o.Identity = "homebrew-cask", s.Application
	o.Desired = "native-auto-updating-app-v1"
	o.RemovalDetail = "remove the signed Zen app at its recorded path; personal profiles remain; a manual reinstall from the same developer at that path is indistinguishable from a browser update"
	o.ApplyDetail = "install the official native app through Homebrew; normal browser updates remain enabled; personal profiles stay untouched"
	present, healthy, err := d.caskIdentity(ctx, s, s.Application)
	unknown := func(err error) (Observation, error) {
		if ctx.Err() != nil {
			return o, ctx.Err()
		}
		o.Unknown, o.Present, o.Healthy = true, false, false
		o.Pending = "native application identity could not be inspected: " + err.Error()
		return o, nil
	}
	if err != nil {
		return unknown(err)
	}
	if !present && !owned {
		// Reuse an authentic real user bundle even when Homebrew already records
		// it at a user appdir. Never acquire its receipt or a symlinked launcher.
		info, statErr := os.Lstat(s.UserApplication)
		if statErr == nil && info.IsDir() {
			_, healthy, err = d.caskIdentity(ctx, s, s.UserApplication)
			if err != nil {
				return unknown(err)
			}
			if healthy {
				present, o.Identity = true, s.UserApplication
			}
		} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return unknown(statErr)
		}
	}
	if o.Present {
		o.Healthy = present && healthy && pkg.Source == d.Packages[r.ID]
		if present && !healthy {
			o.Pending = "the native app changed identity or contains unsigned edits; preserve it and inspect it manually"
		}
		if pkg.Source != d.Packages[r.ID] {
			o.Pending = "the native app uses a different Homebrew source; preserve it"
		}
		if !present {
			o.HealthIssue = "the recorded native application is absent"
			if !owned && o.Pending == "" {
				o.Pending = "your Homebrew zen record has no app; use HOMEBREW_NO_AUTOREMOVE=1 HOMEBREW_NO_INSTALL_CLEANUP=1 brew reinstall --cask zen to restore it, or HOMEBREW_NO_AUTOREMOVE=1 brew uninstall --cask zen before installing it through setup"
			}
		}
		if err := d.caskInstalledRecipe(s, d.Packages[r.ID]); owned && err != nil {
			o.Pending = "preserve the native app: " + err.Error()
		}
	} else {
		if present && !healthy {
			o.ApplyBlocked = "the application target is occupied by an unverified app or link; preserve it"
		}
		o.Present, o.Healthy = present, present && healthy
	}
	if _, err := os.Lstat(s.Binary); err == nil && (owned || !o.Healthy) {
		o.Pending = "preserve the existing Zen command at " + s.Binary + "; Homebrew may unlink it during removal"
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return unknown(err)
	}
	if o.Present {
		o.Fingerprint, err = digest(struct{ Token, Source, Application string }{d.Packages[r.ID], pkg.Source, o.Identity})
	}
	return o, err
}

// Inspect both current public metadata and the installed uninstall recipe.
// Homebrew loads the latter for Remove/Reinstall; current API data alone cannot
// constrain the files or hooks in that saved recipe.
type brewApplicationMetadata struct {
	Token       string                       `json:"token"`
	FullToken   string                       `json:"full_token"`
	Tap         string                       `json:"tap"`
	SHA256      string                       `json:"sha256"`
	AutoUpdates bool                         `json:"auto_updates"`
	DependsOn   map[string]json.RawMessage   `json:"depends_on"`
	Artifacts   []map[string]json.RawMessage `json:"artifacts"`
}

func validateCaskMetadata(data []byte, s brewCaskSpec, token string, installed bool) error {
	var doc brewApplicationMetadata
	if len(data) > 1<<20 || json.Unmarshal(data, &doc) != nil || doc.Token != token || doc.FullToken != token || doc.Tap != "homebrew/cask" {
		return errors.New("native app metadata has an unverified source")
	}
	if !installed && (!operationID.MatchString(doc.SHA256) || !doc.AutoUpdates || len(doc.DependsOn) != 0) {
		return errors.New("native app recipe changed its checksum, updater or dependency contract")
	}
	return validateCaskArtifacts(doc.Artifacts, s, token)
}

func validateCaskArtifacts(artifacts []map[string]json.RawMessage, s brewCaskSpec, token string) error {
	app := false
	for _, artifact := range artifacts {
		kind := ""
		for key := range artifact {
			if key != "target" {
				if kind != "" {
					return errors.New("ambiguous native app artifact")
				}
				kind = key
			}
		}
		var expected any
		switch kind {
		case "app":
			if app {
				return errors.New("duplicate native app artifact")
			}
			app = true
			expected = []string{filepath.Base(s.Application)}
		case "uninstall":
			expected = []map[string]string{{"quit": s.Bundle}}
		case "command_wrapper":
			expected = []any{token, map[string]string{"executable": "$APPDIR/" + filepath.Base(s.Application) + "/Contents/MacOS/" + token}}
		case "zap":
			// Zap is never dispatched. No code artifacts are accepted here.
			var zap []map[string]json.RawMessage
			if json.Unmarshal(artifact[kind], &zap) != nil {
				return errors.New("invalid unused zap metadata")
			}
			for _, item := range zap {
				for key := range item {
					if key != "trash" && key != "rmdir" {
						return errors.New("unsupported zap metadata")
					}
				}
			}
			continue
		default:
			return fmt.Errorf("native app recipe contains unsupported artifact %s", kind)
		}
		var actual any
		if json.Unmarshal(artifact[kind], &actual) != nil {
			return errors.New("invalid native app artifact")
		}
		// brew info expands API path variables; installed receipt artifacts also
		// use physical paths. Both must resolve to this exact reviewed target.
		if kind == "command_wrapper" {
			expanded := []any{token, map[string]string{"executable": filepath.Join(s.Application, "Contents", "MacOS", token)}}
			x, _ := json.Marshal(expanded)
			y, _ := json.Marshal(actual)
			if string(x) == string(y) {
				expected = expanded
			}
		}
		want, _ := json.Marshal(expected)
		got, _ := json.Marshal(actual)
		if string(got) != string(want) {
			return errors.New("native app artifact differs from its reviewed contract")
		}
		if target, ok := artifact["target"]; ok {
			var path string
			if json.Unmarshal(target, &path) != nil {
				return errors.New("invalid native app target")
			}
			if kind == "app" && path != s.Application || kind == "command_wrapper" && path != "$HOMEBREW_PREFIX/bin/"+token && path != s.Binary {
				return errors.New("native app artifact was redirected")
			}
		}
	}
	if !app {
		return errors.New("native app recipe omitted its application")
	}
	return nil
}

func (d *BrewDriver) caskInstalledRecipe(s brewCaskSpec, token string) error {
	root := filepath.Join(s.Caskroom, ".metadata")
	configPath := filepath.Join(root, "config.json")
	bound, err := bindConfigDestination(configPath)
	if err != nil || bound != configPath {
		return errors.Join(errors.New("native app metadata was redirected"), err)
	}
	data, err := readDocument(configPath)
	if err != nil {
		return err
	}
	var config map[string]map[string]json.RawMessage
	if json.Unmarshal(data, &config) != nil || config["default"] == nil {
		return errors.New("native app configuration is incomplete")
	}
	appdir := ""
	for _, layer := range []string{"default", "env", "explicit"} {
		for key, value := range config[layer] {
			if key == "appdir" {
				if json.Unmarshal(value, &appdir) != nil {
					return errors.New("invalid native app directory")
				}
			}
			if (layer == "env" || layer == "explicit") && key != "appdir" && key != "languages" {
				return errors.New("native app configuration has an unsupported override")
			}
		}
	}
	if appdir != filepath.Dir(s.Application) {
		return errors.New("native app install directory changed")
	}
	paths, err := filepath.Glob(filepath.Join(root, "*", "*", "Casks", "*"))
	if err != nil {
		return err
	}
	if len(paths) != 1 || filepath.Base(paths[0]) != token+".json" {
		return errors.New("native app uninstall recipe is ambiguous or executable")
	}
	bound, err = bindConfigDestination(paths[0])
	if err != nil || bound != paths[0] {
		return errors.Join(errors.New("native app recipe was redirected"), err)
	}
	data, err = readDocument(paths[0])
	if err != nil {
		return err
	}
	var compact map[string]json.RawMessage
	if json.Unmarshal(data, &compact) != nil {
		return errors.New("invalid native app installed metadata")
	}
	if len(compact) != 0 {
		return validateCaskMetadata(data, s, token, true)
	}
	// Homebrew 7 stores only URL overrides in Casks/token.json (empty for Zen).
	// The exact removal artifacts and source live in the native tab receipt.
	receiptPath := filepath.Join(root, "INSTALL_RECEIPT.json")
	data, err = readDocument(receiptPath)
	if err != nil {
		return err
	}
	var receipt struct {
		FlightBlocks *bool `json:"uninstall_flight_blocks"`
		Source       struct {
			Tap string `json:"tap"`
		} `json:"source"`
		Artifacts []map[string]json.RawMessage `json:"uninstall_artifacts"`
	}
	if len(data) > 1<<20 || json.Unmarshal(data, &receipt) != nil || receipt.FlightBlocks == nil || *receipt.FlightBlocks || receipt.Source.Tap != "homebrew/cask" {
		return errors.New("native app receipt has an unverified source or executable uninstall hooks")
	}
	return validateCaskArtifacts(receipt.Artifacts, s, token)
}

func (d *BrewDriver) checkCaskMutation(ctx context.Context, intent brewIntent, s brewCaskSpec) error {
	present, healthy, err := d.caskIdentity(ctx, s, s.Application)
	if err != nil {
		return err
	}
	if intent.Action == "remove" {
		if present && !healthy {
			return errors.New("refusing to remove an unverified native app")
		}
	} else if present {
		return errors.New("refusing to overwrite an existing native app; review a fresh plan")
	}
	if intent.Action == "install" {
		if info, err := os.Lstat(s.UserApplication); err == nil && info.IsDir() {
			_, healthy, err := d.caskIdentity(ctx, s, s.UserApplication)
			if err != nil {
				return err
			}
			if healthy {
				return errors.New("an existing user application appeared; reuse it after a fresh preview")
			}
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if _, err := os.Lstat(s.Binary); !errors.Is(err, os.ErrNotExist) {
		return errors.Join(errors.New("preserve the existing native app command"), err)
	}
	output, err := d.Query(ctx, false, "/usr/bin/osascript", nil, "-l", "JavaScript", "-e", legacyMacApplicationScript, s.Bundle)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(output)) != "not-running" {
		return errors.New("quit all copies of Zen before changing its installation")
	}
	installed, err := brewInventory(ctx, d.Query)
	if err != nil {
		return err
	}
	if _, exists := installed["cask/"+intent.Package]; exists {
		if err := d.caskInstalledRecipe(s, intent.Package); err != nil {
			return err
		}
	}
	if intent.Action == "remove" {
		return nil
	}
	output, err = d.Query(ctx, false, "brew", nil, "info", "--json=v2", "--cask", intent.Package)
	if err != nil {
		return err
	}
	var doc struct {
		Casks []json.RawMessage `json:"casks"`
	}
	if len(output) > 1<<20 || json.Unmarshal(output, &doc) != nil || len(doc.Casks) != 1 {
		return errors.New("native app metadata is incomplete")
	}
	return validateCaskMetadata(doc.Casks[0], s, intent.Package, false)
}
