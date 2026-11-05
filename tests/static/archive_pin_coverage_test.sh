#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."
python3 scripts/update-agent-cli-pins.py --check-coverage
ruby -ryaml -rjson -ropen3 -rtmpdir -rfileutils - <<'RUBY'
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
        [ "$STUB_BRANCH" = present ] && { echo 1111111111111111111111111111111111111111; exit 0; }
        [ "$STUB_BRANCH" = absent ] && { echo 'gh: Not Found (HTTP 404)' >&2; exit 1; }
        echo 'gh: Bad credentials (HTTP 401)' >&2; exit 1 ;;
      */compare/main...1111111111111111111111111111111111111111)
        [ "$3" = --jq ] || exit 9
        exec jq -r "$4" <<<"$STUB_COMPARE" ;;
    esac
    exit 9
  GH
  File.chmod(0o755, File.join(directory, 'gh'))
  bot = { 'login' => 'dotfiles-pins[bot]' }
  person = { 'login' => 'luisgui1757' }
  [
    ['absent', [], 0, ''],
    ['present', [{ 'author' => bot, 'committer' => bot }], 0, '1111111111111111111111111111111111111111'],
    ['present', [{ 'author' => bot, 'committer' => bot }, { 'author' => person, 'committer' => person }], 1, nil],
    ['present', [{ 'author' => bot, 'committer' => person }], 1, nil],
    ['present', [{ 'author' => nil, 'committer' => nil }], 1, nil],
    ['error', [], 1, nil]
  ].each do |branch, commits, expected, sha|
    output_file = File.join(directory, 'output')
    File.write(output_file, '')
    environment = { 'PATH' => "#{directory}:#{ENV['PATH']}", 'RUNNER_TEMP' => directory, 'GH_TOKEN' => 'token',
                    'APP_SLUG' => 'dotfiles-pins', 'BASE' => 'main', 'BRANCH' => 'automation/agent-cli-pins',
                    'GITHUB_REPOSITORY' => 'owner/dotfiles', 'GITHUB_OUTPUT' => output_file, 'STUB_BRANCH' => branch,
                    'STUB_COMPARE' => JSON.generate({ 'commits' => commits }) }
    output, status = Open3.capture2e(environment, 'bash', '-c', guard.fetch('run'))
    abort "FAIL: proposal branch #{branch} with #{commits}: #{output}" unless status.exitstatus == expected
    abort "FAIL: proposal branch #{branch} recorded the wrong SHA" unless sha.nil? || File.read(output_file) == "sha=#{sha}\n"
  end
end
puts 'OK: the pin workflow keeps commits from other authors and committers on its proposal branch'

publish = steps.find { |step| step['name'] == 'Open or update the pull request' }
Dir.mktmpdir do |directory|
  File.write(File.join(directory, 'gh'), <<~'GH')
    #!/usr/bin/env bash
    case "$1 $2" in
      'api /users/'*) echo 41 ;;
      'auth setup-git'|'pr create'|'pr edit') ;;
      'pr list') ;;
      *) exit 9 ;;
    esac
  GH
  File.chmod(0o755, File.join(directory, 'gh'))
  git = lambda do |*arguments, chdir:|
    output, status = Open3.capture2e('git', '-c', 'user.name=t', '-c', 'user.email=t@example.com', *arguments, chdir: chdir)
    abort "FAIL: git #{arguments.join(' ')}: #{output}" unless status.success?
    output.strip
  end
  remote = File.join(directory, 'remote.git')
  git.call('init', '-q', '--bare', '-b', 'main', remote, chdir: directory)
  [
    ['absent', '', 0],
    ['moved', 'stale', 1],
    ['present', 'current', 0],
    ['appeared', '', 1]
  ].each do |state, lease, expected|
    work = Dir.mktmpdir(nil, directory)
    git.call('init', '-q', '-b', 'main', work, chdir: directory)
    FileUtils.mkdir_p(File.join(work, 'installer'))
    File.write(File.join(work, 'installer/archive-pins.json'), "{}\n")
    git.call('add', '.', chdir: work)
    git.call('commit', '-q', '-m', 'base', chdir: work)
    git.call('push', '-q', '--force', remote, 'main', chdir: work)
    git.call('push', '-q', remote, ':refs/heads/automation/agent-cli-pins', chdir: work) if git.call('ls-remote', remote, chdir: work).include?('automation')
    reviewer = nil
    unless state == 'absent'
      File.write(File.join(work, 'review.txt'), "#{state}\n")
      git.call('add', 'review.txt', chdir: work)
      git.call('commit', '-q', '-m', 'reviewer', chdir: work)
      reviewer = git.call('rev-parse', 'HEAD', chdir: work)
      git.call('push', '-q', remote, 'HEAD:refs/heads/automation/agent-cli-pins', chdir: work)
      git.call('reset', '-q', '--hard', 'HEAD~1', chdir: work)
    end
    expected_sha = { '' => '', 'stale' => '2' * 40, 'current' => reviewer }.fetch(lease)
    git.call('remote', 'add', 'origin', remote, chdir: work)
    File.write(File.join(work, 'installer/archive-pins.json'), "{\"new\": true}\n")
    File.write(File.join(directory, 'summary.md'), "Archive pins:\n")
    environment = { 'PATH' => "#{directory}:#{ENV['PATH']}", 'RUNNER_TEMP' => directory, 'GH_TOKEN' => 'token',
                    'APP_SLUG' => 'dotfiles-pins', 'BASE' => 'main', 'BRANCH' => 'automation/agent-cli-pins',
                    'EXPECTED' => expected_sha, 'GIT_CONFIG_GLOBAL' => File::NULL }
    output, status = Open3.capture2e(environment, 'bash', '-c', publish.fetch('run'), chdir: work)
    abort "FAIL: publication with a #{state} proposal branch: #{output}" unless status.exitstatus == expected
    after = git.call('ls-remote', remote, 'refs/heads/automation/agent-cli-pins', chdir: work).split.first
    abort "FAIL: publication replaced a reviewer commit on a #{state} branch" if expected == 1 && state == 'moved' && after != reviewer
  end
end
puts 'OK: the pin workflow pushes only over the proposal branch state that it checked'
RUBY
