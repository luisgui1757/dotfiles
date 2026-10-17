package installer

import (
	"context"
	"errors"
	"path/filepath"
)

// PythonPinID is filled from the selected Python archive at construction. A
// Python update therefore changes this recipe's desired identity as well.
type Latex2textPin struct {
	Backend     ArchivePin `json:"backend"`
	PythonPinID string     `json:"python_pin_id"`
	// Zero is readable historical provenance, never a new preparation.
	ConsoleRevision int `json:"console_revision,omitempty"`
}

func validateLatex2textPin(pin ArchivePin) error {
	recipe := pin.Latex2text
	if recipe == nil || pin.Format != "file" || pin.File != "pylatexenc-"+pin.Version+".tar.gz" || !pythonToolVersion.MatchString(pin.Version) || !operationID.MatchString(recipe.PythonPinID) || recipe.ConsoleRevision < 0 || recipe.ConsoleRevision > 1 {
		return errors.New("latex2text requires a pinned source distribution and Python archive identity")
	}
	backend := recipe.Backend
	if len(backend.RustComponents) > 0 || backend.RustTarget != "" || backend.Latex2text != nil || backend.Yamllint != nil || backend.PythonStdlib != "" || backend.PortableGit || backend.Format != "file" || backend.File != "setuptools-"+backend.Version+"-py3-none-any.whl" || !pythonToolVersion.MatchString(backend.Version) || len(backend.Commands) != 0 {
		return errors.New("latex2text requires its pinned setuptools wheel")
	}
	return backend.Validate()
}

func (d *ArchiveDriver) prepareLatex2text(ctx context.Context, intent archiveIntent, payload string) error {
	pin := intent.Pin
	if err := validateLatex2textPin(pin); err != nil {
		return err
	}
	if pin.Latex2text.ConsoleRevision != 1 {
		return errors.New("saved latex2text preparation predates UTF-8 console support; preserve the original operation and payload for explicit recovery")
	}
	backend := pin.Latex2text.Backend
	backendDirectory := filepath.Join(payload, ".backend")
	if err := downloadArchive(ctx, d.Client, backend, backendDirectory); err != nil {
		return err
	}
	requirements := []privatePythonRequirement{
		{"setuptools", filepath.Join(backendDirectory, backend.File), backend.SHA256, false},
		{"pylatexenc", filepath.Join(payload, pin.File), pin.SHA256, true},
	}
	probe := "import importlib.metadata,sys; from pylatexenc.latex2text import LatexNodes2Text; assert importlib.metadata.version('pylatexenc') == sys.argv[1]; assert LatexNodes2Text().latex_to_text(r'\\alpha') == 'α'"
	return d.preparePrivatePython(ctx, intent, payload, "latex2text", requirements, probe, pin.Version)
}

// Reuse the pinned pip launcher writer on each platform, including its native
// Windows .exe. Configure the command's streams before upstream argument parsing
// and conversion; GUI/piped Neovim use must not depend on the parent code page.
const latex2textConsole = `import os, sys
from pip._vendor.distlib.scripts import ScriptMaker
maker = ScriptMaker(None, os.path.dirname(sys.executable))
maker.clobber = True
maker.variants = {""}
maker.executable = sys.executable
maker.script_template = """# -*- coding: utf-8 -*-
import sys
from %(module)s import %(import_name)s
if __name__ == '__main__':
	for stream in (sys.stdin, sys.stdout, sys.stderr):
		stream.reconfigure(encoding='utf-8')
	sys.exit(%(func)s())
"""
maker.make("latex2text = pylatexenc.latex2text.__main__:main")
`
