package installer

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// Native maintenance may rebuild or update an existing package; it must not
// acquire it or change its source, pin or user/manual classification.
type brewClassification struct {
	Source    string `json:"source"`
	Held      bool   `json:"held"`
	Automatic bool   `json:"automatic"`
}

func brewClassificationOf(pkg nativePackage) brewClassification {
	return brewClassification{Source: pkg.Source, Held: pkg.Held, Automatic: pkg.Automatic}
}

func (c brewClassification) validate(name string) error {
	short, err := brewShortName(c.Source)
	if err != nil || short != strings.TrimPrefix(name, "cask/") || !brewPackageName.MatchString(name) && !validBrewCaskIdentity(name) {
		return errors.New("invalid saved Homebrew package classification")
	}
	return nil
}

func (d *BrewDriver) verifyPreexisting(intent brewIntent, installed map[string]nativePackage) error {
	touched, removed := map[string]bool{}, map[string]bool{}
	for _, command := range intent.Commands {
		reply, err := d.commandResult(command)
		if err != nil {
			return err
		}
		kegs, err := brewInstallEvidence(reply.Output, filepath.ToSlash(d.Cellar), command.Operation)
		if err != nil {
			return err
		}
		for _, keg := range kegs {
			touched[keg.Name] = true
		}
		if command.Arguments[0] == "uninstall" {
			for _, name := range command.Arguments[3:] {
				removed[name] = true
			}
		}
	}
	for name, baseline := range intent.Before {
		current, present := installed[name]
		if !present && !removed[name] {
			return fmt.Errorf("pre-existing Homebrew package %s disappeared during native maintenance", name)
		}
		if present && touched[name] && brewClassificationOf(baseline) != brewClassificationOf(current) {
			return fmt.Errorf("pre-existing Homebrew package %s changed source, pin or manual classification during native maintenance", name)
		}
	}
	return nil
}

func validateBrewBaseline(name string, pkg nativePackage) error {
	if pkg.Name != name || pkg.Version == "" {
		return errors.New("invalid saved Homebrew native baseline")
	}
	if err := brewClassificationOf(pkg).validate(name); err != nil {
		return err
	}
	for _, dependency := range pkg.Dependencies {
		if !brewPackageName.MatchString(dependency) {
			return errors.New("invalid saved Homebrew runtime dependency")
		}
	}
	return nil
}
