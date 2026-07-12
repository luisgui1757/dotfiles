#!/usr/bin/env ruby
# frozen_string_literal: true

require "yaml"

if ARGV.length != 1
  warn "usage: assert_no_probot_branches.rb <settings.yml>"
  exit 2
end

path = ARGV.fetch(0)
begin
  source = File.read(path, encoding: "UTF-8")
  if source.bytesize > 65_536
    warn "FAIL: #{path} exceeds the bounded Settings policy input size"
    exit 1
  end
  settings = YAML.safe_load(
    source,
    permitted_classes: [],
    permitted_symbols: [],
    aliases: true,
    filename: path
  )
rescue Psych::Exception, SystemCallError => e
  warn "FAIL: cannot safely parse #{path}: #{e.message}"
  exit 1
end

unless settings.is_a?(Hash)
  warn "FAIL: #{path} must contain one top-level YAML mapping"
  exit 1
end

if settings.key?("branches")
  warn "FAIL: #{path} must not define the top-level branches key; branch protection belongs to the transactional safeguard script"
  exit 1
end
