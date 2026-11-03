#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."
python3 scripts/update-agent-cli-pins.py --check-coverage
ruby -ryaml -ropen3 -rtmpdir - <<'RUBY'
steps = YAML.load_file('.github/workflows/agent-cli-pins.yml')['jobs']['propose']['steps']
cases = [
  ['Report a proposal without the App', '0', 1],
  ['Fail when a release needs manual review', '1', 1],
  ['Fail when a release needs manual review', '', 1],
  ['Fail when a release needs manual review', '0', 0]
]
Dir.mktmpdir do |directory|
  cases.each do |name, code, expected|
    script = steps.find { |step| step['name'] == name }.fetch('run')
    output, status = Open3.capture2e({'RUNNER_TEMP' => directory, 'BUMP_STATUS' => code}, 'bash', '-c', script)
    abort "FAIL: #{name} with status #{code}: #{output}" unless status.exitstatus == expected
  end
end
puts 'OK: the pin workflow fails for missing App access and failed verification'
RUBY
