#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."
python3 scripts/update-agent-cli-pins.py --check-coverage
ruby -ryaml -rjson -ropen3 -rtmpdir - <<'RUBY'
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

guard = steps.find { |step| step['name'] == 'Keep commits from other authors on the proposal branch' }
abort 'FAIL: the pin workflow does not protect commits from other authors' unless guard
Dir.mktmpdir do |directory|
  File.write(File.join(directory, 'gh'), <<~'GH')
    #!/usr/bin/env bash
    case "$2" in
      */branches/*)
        [ "$STUB_BRANCH" = present ] && exit 0
        [ "$STUB_BRANCH" = absent ] && { echo 'gh: Not Found (HTTP 404)' >&2; exit 1; }
        echo 'gh: Bad credentials (HTTP 401)' >&2; exit 1 ;;
      */compare/*)
        [ "$3" = --jq ] || exit 9
        exec jq -r "$4" <<<"$STUB_COMPARE" ;;
    esac
    exit 9
  GH
  File.chmod(0o755, File.join(directory, 'gh'))
  bot = { 'author' => { 'login' => 'dotfiles-pins[bot]' } }
  person = { 'author' => { 'login' => 'luisgui1757' } }
  unlinked = { 'author' => nil }
  [
    ['absent', [], 0],
    ['present', [bot], 0],
    ['present', [bot, person], 1],
    ['present', [unlinked], 1],
    ['error', [], 1]
  ].each do |branch, commits, expected|
    environment = { 'PATH' => "#{directory}:#{ENV['PATH']}", 'RUNNER_TEMP' => directory, 'GH_TOKEN' => 'token',
                    'APP_SLUG' => 'dotfiles-pins', 'BASE' => 'main', 'BRANCH' => 'automation/agent-cli-pins',
                    'GITHUB_REPOSITORY' => 'owner/dotfiles', 'STUB_BRANCH' => branch,
                    'STUB_COMPARE' => JSON.generate({ 'commits' => commits }) }
    output, status = Open3.capture2e(environment, 'bash', '-c', guard.fetch('run'))
    abort "FAIL: proposal branch #{branch} with #{commits.length} commits: #{output}" unless status.exitstatus == expected
  end
end
puts 'OK: the pin workflow keeps commits from other authors on its proposal branch'
RUBY
