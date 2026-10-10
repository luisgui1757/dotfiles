package installer

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// NewNativeController is side-effect free. Folder discovery is separate so
// native lifecycle tests can exercise the real application in a disposable home.
func NewNativeController(repository, source, stateDirectory string, platform NativePlatform, folders ConfigFolders) (Controller, error) {
	target := platform.Context
	var controller Controller
	var err error
	if err := target.Validate(); err != nil {
		return controller, err
	}
	if !filepath.IsAbs(repository) || !filepath.IsAbs(stateDirectory) {
		return controller, errors.New("checkout and state directory must be absolute")
	}
	repository, err = resolveConfigPath(repository)
	if err != nil {
		return controller, err
	}
	bound, err := bindConfigDestination(filepath.Join(stateDirectory, "state.json"))
	if err != nil {
		return controller, err
	}
	stateDirectory = filepath.Dir(bound)
	catalog, err := DefaultCatalog()
	if err != nil {
		return controller, err
	}
	manifest, err := DefaultConfigManifest(catalog)
	if err != nil {
		return controller, err
	}
	pins, err := DefaultArchivePins(platform)
	if err != nil {
		return controller, err
	}
	archives := &ArchiveDriver{Directory: filepath.Join(stateDirectory, "packages"), Pins: pins}
	archives.DpkgDeb = platform.DpkgDeb
	archives.WindowsRustLauncher = platform.WindowsRustLauncher
	configurations := &ConfigDriver{Catalog: catalog, Manifest: manifest, Target: target, Folders: folders, Repository: repository, Directory: stateDirectory}
	profiles := &ProfileDriver{Directory: stateDirectory, Windows: target.OS == "windows", Targets: map[string][]ProfileTarget{}}
	configTargets, err := manifest.Resolve(catalog, target, folders, repository, "config.starship")
	if err != nil {
		return controller, err
	}
	if len(configTargets) != 1 {
		return controller, errors.New("Starship requires exactly one configuration file")
	}
	err = configureShellProfiles(profiles, target, folders, archives, configTargets[0].Destination, nil)
	if err != nil {
		return controller, err
	}
	if err := configureTmuxProfiles(profiles, target, folders, repository, archives); err != nil {
		return controller, err
	}
	configurePiSettings(profiles, folders)
	configureVSCodeSettings(profiles, target, folders)
	windowsPowerShell := ""
	if target.OS == "windows" {
		windowsPowerShell, err = archives.CommandPath("tool.powershell", "pwsh")
		if err != nil {
			return controller, err
		}
	}
	if err := configureWindowsTerminal(profiles, target, folders, repository, windowsPowerShell); err != nil {
		return controller, err
	}
	if err := configureSentinel(profiles, folders, platform.Sentinel); err != nil {
		return controller, err
	}
	statePath := filepath.Join(stateDirectory, "state.json")
	driver := &NativeDriver{Catalog: catalog, Context: target, Archives: archives, Configurations: configurations, Profiles: profiles, HomebrewIssue: platform.HomebrewIssue, APTIssue: platform.APTIssue,
		StatePath: statePath, session: newNativeSession(filepath.Join(stateDirectory, "native", "worker"))}
	driver.LinuxClipboard = platform.LinuxClipboard
	archives.Run = driver.session.run
	driver.Desktop = &DesktopDriver{Target: target, Folders: folders, Directory: stateDirectory, Archives: archives, PowerShell: platform.WindowsPowerShell, Run: driver.session.run}
	driver.Fonts = &FontDriver{Target: target, Folders: folders, Directory: stateDirectory, Archives: archives, PowerShell: platform.WindowsPowerShell, Run: driver.session.run, FontMatch: "/usr/bin/fc-match", FontCache: "/usr/bin/fc-cache"}
	pythonCommand := "python3"
	if target.OS == "windows" {
		pythonCommand = "python"
	}
	archives.Python, err = archives.CommandPath("tool.python", pythonCommand)
	if err != nil {
		return controller, err
	}
	driver.Brew, err = configureHomebrew(platform, catalog, driver.session)
	if err != nil {
		return controller, err
	}
	driver.Homebrew, err = configureHomebrewBootstrap(platform, folders, driver.session)
	if err != nil {
		return controller, err
	}
	driver.ApplePrerequisites, err = configureMacOSPrerequisites(platform)
	if err != nil {
		return controller, err
	}
	driver.AppleCLT, err = configureAppleCLT(platform, folders, driver.session)
	if err != nil {
		return controller, err
	}
	if target.OS == "linux" {
		driver.APT, err = configureLinuxAPT(platform.APT, catalog, driver.session)
		if err != nil {
			return controller, err
		}
	}
	driver.WindowsVendor, err = configureWindowsVendor(platform, driver.session)
	if err != nil {
		return controller, err
	}
	driver.WindowsVendorIssue = platform.WindowsVendorIssue
	driver.BuildTools, err = configureWindowsBuildTools(platform, driver.session)
	if err != nil {
		return controller, err
	}
	driver.NvimSync, err = configureNvimSync(repository, stateDirectory, platform, folders, catalog, archives, driver.session)
	if err != nil {
		return controller, err
	}
	if driver.BuildTools != nil {
		driver.NvimSync.Run = withCompilerEnvironment(driver.BuildTools.CompilerEnvironment, driver.session.run)
	}
	driver.shellSelection = func(selected []string, receipt Receipt) (*ProfileDriver, error) {
		bound := &ProfileDriver{Directory: profiles.Directory, Windows: profiles.Windows, Targets: map[string][]ProfileTarget{}}
		err := configureShellProfiles(bound, target, folders, archives, configTargets[0].Destination, selected, homebrewCommandDirectories(platform, catalog, selected)...)
		if err == nil && target.OS != "windows" {
			err = retainBashLoginTarget(bound, folders.Home, receipt)
		}
		if err == nil {
			err = configureTmuxProfiles(bound, target, folders, repository, archives)
		}
		configurePiSettings(bound, folders)
		configureVSCodeSettings(bound, target, folders)
		if err == nil {
			err = configureWindowsTerminal(bound, target, folders, repository, windowsPowerShell)
		}
		if err == nil {
			err = configureSentinel(bound, folders, platform.Sentinel)
		}
		return bound, err
	}
	return Controller{Catalog: catalog, Context: target, Source: source, Home: folders.Home, StatePath: statePath, Driver: driver}, nil
}

func shellLiteral(s string) string      { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
func powershellLiteral(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func DiscoverStateDirectory(target Context, folders ConfigFolders) (string, error) {
	root := os.Getenv("XDG_STATE_HOME")
	if target.OS == "windows" {
		root = folders.LocalAppData
	} else if root == "" {
		root = filepath.Join(folders.Home, ".local", "state")
	}
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || strings.ContainsAny(root, "\x00\r\n") {
		return "", errors.New("installer state folder must be an absolute canonical path")
	}
	return filepath.Join(root, "dotfiles"), nil
}
