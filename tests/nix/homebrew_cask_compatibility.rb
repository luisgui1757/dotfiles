# Run through the installed Homebrew: brew ruby <this file>.
# No apps are downloaded, installed, launched or removed by these probes.
require "cask/cask"
require "cask/artifact"
require "install_steps"

failed = false

# AeroSpace interpolates its version inside postflight_steps. The old runtime
# accepted the stanza but could not serialize that interpolation.
begin
  steps = Homebrew::InstallSteps::DSL.build do
    run "/usr/bin/true", args: ["AeroSpace-v#{version}/bin/aerospace"]
  end
  unless steps.fetch(0).fetch("args") == ["AeroSpace-v{{version}}/bin/aerospace"]
    raise "postflight version interpolation was not preserved"
  end
  puts "ok  : Homebrew serializes cask postflight versions"
rescue StandardError => e
  warn "FAIL: cask postflight versions: #{e.message}"
  failed = true
end

# API artifact order is not an installation-order guarantee. Ghostty links
# completions from inside its app, so Homebrew must move the app first even
# when the completion arrives before the app in serialized metadata.
%i[bash_completion zsh_completion fish_completion].each do |completion|
  cask = Cask::Cask.new("dotfiles-completion-probe") do
    version "1.0"
    sha256 :no_check
    url "https://example.invalid/dotfiles-completion-probe.dmg"
    public_send completion, "#{appdir}/Probe.app/Contents/Resources/completion"
    app "Probe.app"
  end
  artifacts = cask.artifacts.to_a.sort
  if artifacts.length == 2 && artifacts.first.is_a?(Cask::Artifact::App)
    puts "ok  : Homebrew installs the app before #{completion}"
  else
    warn "FAIL: Homebrew installs #{completion} before its source app"
    failed = true
  end
end

exit(failed ? 1 : 0)
