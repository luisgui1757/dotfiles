#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

ruby - <<'RUBY'
require 'yaml'

load_workflow = ->(name) { YAML.load_file(".github/workflows/#{name}.yml") }
test = load_workflow.call('test')
ubuntu = test.fetch('jobs').fetch('ubuntu')
install = ubuntu.fetch('steps').find { |step| step['name'] == 'Install deps' }.fetch('run')
abort 'FAIL: hosted PowerShell must not use the unavailable Microsoft apt package' if
  install.include?('packages.microsoft.com') || install.include?('apt-get install -y powershell')
abort 'FAIL: hosted PowerShell must be required, not silently skipped' unless
  install.lines.any? { |line| line.strip == 'command -v pwsh' }
abort 'FAIL: hosted PowerShell must execute and require major version 7 or newer' unless
  install.lines.any? { |line| line.strip == %q(pwsh -NoLogo -NoProfile -Command 'if ($PSVersionTable.PSVersion.Major -lt 7) { throw "PowerShell 7 is required" }; $PSVersionTable.PSVersion') }

# Runner updates must not rename compatibility checks consumed by release
# certification or mislabel the producer in the exact-run proof artifact.
{'nix' => 'flake-check', 'e2e-install' => 'setup-sh'}.each do |workflow, job_name|
  job = load_workflow.call(workflow).fetch('jobs').fetch(job_name)
  job.fetch('strategy').fetch('matrix').fetch('include').each do |row|
    rendered = job.fetch('name').gsub(/\$\{\{ matrix\.(\w+) \}\}/) { row.fetch(Regexp.last_match(1)) }
    abort "FAIL: #{workflow} producer #{rendered} differs from its proof identity" unless
      rendered == row.fetch('legacy_context')
  end
end
puts 'OK: Hosted PowerShell and rendered producer identities match their contracts'
RUBY
