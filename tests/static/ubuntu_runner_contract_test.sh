#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

ruby - <<'RUBY'
require 'yaml'

load_workflow = ->(name) { YAML.load_file(".github/workflows/#{name}.yml") }
test = load_workflow.call('test')
ubuntu = test.fetch('jobs').fetch('ubuntu')
version = ubuntu.fetch('runs-on').delete_prefix('ubuntu-')
install = ubuntu.fetch('steps').find { |step| step['name'] == 'Install deps' }.fetch('run')
expected_url = "https://packages.microsoft.com/config/ubuntu/#{version}/packages-microsoft-prod.deb"
abort 'FAIL: Microsoft repository package does not match the Ubuntu runner' unless install.include?(expected_url)
abort 'FAIL: deleted Microsoft keyring must be restored without a conffile prompt' unless
  install.include?('sudo dpkg --force-confmiss -i /tmp/packages-microsoft-prod.deb')

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
puts 'OK: Ubuntu repository and rendered producer identities match their contracts'
RUBY
