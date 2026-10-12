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

puts 'OK: hosted PowerShell requires the actual runner runtime'
RUBY
