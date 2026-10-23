package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const desktopCodeOriginal = "Personal document before native editing.\n"
const desktopCodeEdited = "Saved by the configured editor — café.\n"

type desktopCodeWitness struct {
	Path, Document string
	Mode           os.FileMode
}

func newDesktopCodeWitness(t *testing.T) desktopCodeWitness {
	t.Helper()
	directory := t.TempDir()
	document := filepath.Join(directory, "personal document ü.txt")
	writeConfigFixture(t, document, desktopCodeOriginal)
	if err := os.Chmod(document, 0640); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(document)
	if err != nil {
		t.Fatal(err)
	}
	return desktopCodeWitness{filepath.Join(directory, "Code settings witness.json"), document, info.Mode()}
}

func (w desktopCodeWitness) verify(data []byte) error {
	var result struct{ Theme, Font, TerminalFont, UpdateMode, Before, Reopened, Error string }
	if err := Decode(data, &result); err != nil {
		return err
	}
	if result.Error != "" {
		return fmt.Errorf("Code document journey: %s", result.Error)
	}
	if result.Theme != "Rosé Pine" || !strings.Contains(result.Font, "Hack Nerd Font") || !strings.Contains(result.TerminalFont, "Hack Nerd Font") || result.UpdateMode != "none" {
		return fmt.Errorf("Code did not consume managed settings: %+v", result)
	}
	if result.Before != desktopCodeOriginal || result.Reopened != desktopCodeEdited {
		return fmt.Errorf("Code did not edit/save/close/reopen its document: %+v", result)
	}
	contents, err := os.ReadFile(w.Document)
	if err != nil {
		return err
	}
	info, err := os.Stat(w.Document)
	if err != nil {
		return err
	}
	if string(contents) != desktopCodeEdited || info.Mode() != w.Mode {
		return fmt.Errorf("Code saved document contents or permissions differ from the expected native result")
	}
	return nil
}

func (w desktopCodeWitness) extension() string {
	inputs, _ := json.Marshal(struct{ Witness, Document, Original, Edited string }{w.Path, w.Document, desktopCodeOriginal, desktopCodeEdited})
	return `const input = ` + string(inputs) + `;
const vscode = require('vscode');
const fs = require('fs');
function publish(result) {
	fs.writeFileSync(input.Witness + '.next', JSON.stringify(result));
	fs.renameSync(input.Witness + '.next', input.Witness);
}
exports.activate = async () => {
	try {
		const uri = vscode.Uri.file(input.Document);
		// Opening through the editor avoids openTextDocument's retained model
		// reference, which can outlive tab closure by minutes.
		await vscode.commands.executeCommand('vscode.open', uri, {preview:false});
		const editor = vscode.window.activeTextEditor;
		if (!editor || editor.document.uri.toString() !== uri.toString()) throw new Error('Document did not open in the active editor');
		const document = editor.document;
		const before = document.getText();
		if (before !== input.Original) throw new Error('Initial document does not match the personal baseline');
		const changed = await editor.edit(edit => edit.replace(new vscode.Range(0, 0, document.lineCount, 0), input.Edited));
		if (!changed || !document.isDirty) throw new Error('Editor did not apply the edit');
		if (!await document.save() || document.isDirty) throw new Error('Editor did not save the document');
		if (fs.readFileSync(input.Document, 'utf8') !== input.Edited) throw new Error('Saved bytes differ');
		// Observe disposal before reopening: a second lookup of a cached buffer is
		// not proof that the saved document can be reopened from disk.
		await new Promise((resolve, reject) => {
			const timer = setTimeout(() => { subscription.dispose(); reject(new Error('Document did not close')); }, 10000);
			const subscription = vscode.workspace.onDidCloseTextDocument(closed => {
				if (closed === document) { clearTimeout(timer); subscription.dispose(); resolve(); }
			});
			vscode.commands.executeCommand('workbench.action.closeActiveEditor').then(undefined, error => {
				clearTimeout(timer); subscription.dispose(); reject(error);
			});
		});
		const reopened = await vscode.workspace.openTextDocument(uri);
		await vscode.window.showTextDocument(reopened);
		if (reopened === document || reopened.isDirty || reopened.getText() !== input.Edited) throw new Error('Fresh document did not reload saved bytes');
		const c = vscode.workspace.getConfiguration();
		publish({Theme:c.get('workbench.colorTheme'), Font:c.get('editor.fontFamily'), TerminalFont:c.get('terminal.integrated.fontFamily'), UpdateMode:c.get('update.mode'), Before:before, Reopened:reopened.getText()});
	} catch (error) { publish({Error:String(error)}); }
};
`
}
