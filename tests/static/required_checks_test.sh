#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."
ruby - <<'RUBY'
require 'yaml'
require 'json'
metadata = JSON.parse(File.read('.github/check-identities.json'))
abort 'unsupported required-check cutover schema' unless metadata['schema'] == 3 && metadata['stage'] == 'installer-required-pending-apply'
rendered = []
%w[test installer-engine].each do |name|
  YAML.load_file(".github/workflows/#{name}.yml").fetch('jobs').each do |id, job|
    matrix = job.dig('strategy', 'matrix')
    rows = [{}]
    if matrix
      dimensions = matrix.reject { |key, _| %w[include exclude].include?(key) }
      dimensions.each { |key, values| rows = rows.flat_map { |row| values.map { |value| row.merge(key => value) } } }
      rows = [] if dimensions.empty?
      rows += matrix.fetch('include', [])
      rows -= matrix.fetch('exclude', [])
    end
    rows.each do |row|
      rendered << job.fetch('name', id).gsub(/\$\{\{ matrix\.(\w+) \}\}/) { row.fetch(Regexp.last_match(1)).to_s }
    end
  end
end
abort "required contexts differ from emitted jobs: #{(rendered - metadata['required']) + (metadata['required'] - rendered)}" unless rendered.sort == metadata['required'].sort
abort 'duplicate emitted context' unless rendered.uniq == rendered
rules = JSON.parse(File.read('.github/rulesets/main-integrity.json')).fetch('rules')
checks = rules.find { |rule| rule['type'] == 'required_status_checks' }.fetch('parameters').fetch('required_status_checks')
abort 'ruleset contexts drifted' unless checks == metadata['required'].map { |name| {'context' => name, 'integration_id' => 15368} }
script = File.read('scripts/apply-repo-safeguards.sh')
contexts = script.match(/required_check_contexts\(\) \{\n    cat <<'EOF'\n(.*?)\nEOF/m)[1].lines.map(&:strip)
abort 'safeguard contexts drifted' unless contexts == metadata['required']
%w[build_classic_payload_from_file build_classic_state_from_file verify_local_boundary verify_snapshot_unchanged].each do |boundary|
  abort "missing #{boundary}" unless script.include?(boundary)
end
puts 'OK: emitted jobs, desired ruleset and frozen safeguard apply agree; live policy application remains explicit'
RUBY
ruby tests/static/assert_no_probot_branches.rb .github/settings.yml
