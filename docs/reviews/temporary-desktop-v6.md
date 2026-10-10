# Temporary native desktop verification

Based on delivery `de769314978411d63e509d260ae1e7ce51437f58`, this
branch composes the frozen six-file desktop fixture correction. It has no PR
and grants no merge, release or repository-policy authority.

Two disposable native runners use the canonical Apple Silicon macOS and Windows
targets, pinned Actions/Go, read-only repository permissions and explicit fixture
opt-ins. Real child environment absence, native fonts and the complete published
desktop singleton/group lifecycle remain strict. The application test retains
native-window, settings-witness, integrity, update, adoption and restoration
assertions; launch-return metadata alone cannot make it pass.

The new workflow and this note remain outside delivery. Their results certify
only this diagnostic source; final acceptance still needs all 23 canonical jobs
on the final PR head. No daily-workstation package, font or GUI mutation is
authorized or performed by this diagnostic.

## Native correction follow-up

At9ffa038d, macOS complete font/desktop lifecycle passes; Windows Code singleton
window/settings and fonts pass, then complete group stops at missing Terminal
archive binding. This temporary follow-up uses the consolidated catalog and the
new all-pin routing regression with that exact existing-provider binding. No
window, settings, update, removal or baseline assertion is changed.
