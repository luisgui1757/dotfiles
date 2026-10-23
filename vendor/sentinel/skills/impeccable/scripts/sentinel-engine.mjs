#!/usr/bin/env node
// Sentinel's pinned setup and dispatch wrapper; upstream launchers stay untouched.
import fs from 'node:fs/promises';
import path from 'node:path';
import os from 'node:os';
import { fileURLToPath } from 'node:url';
import { createHash, randomUUID } from 'node:crypto';
import { spawn } from 'node:child_process';

const base = fileURLToPath(new URL('../', import.meta.url));
const lock = JSON.parse(await fs.readFile(new URL('sentinel-engine.json', import.meta.url), 'utf8'));
const platform = `${process.platform}-${process.arch}`;
const asset = lock.assets[platform];
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
const args = process.argv.slice(2);
// The engine prints this command for follow-up actions. Quote for the host shell.
const quote = value => "'" + value.replaceAll("'", process.platform === 'win32' ? "''" : "'\"'\"'") + "'";
const selfCommand = `${process.platform === 'win32' ? '& ' : ''}${quote(process.execPath)} ${quote(fileURLToPath(import.meta.url))}`;
const setup = args.length === 1 && args[0] === '--setup';

try {
  if (!asset) throw new Error(`no pinned Impeccable engine for ${platform}`);
  const root = path.resolve(process.env.IMPECCABLE_HOME || path.join(os.homedir(), '.impeccable'));
  const folder = path.join(root, 'sentinel', lock.version, asset.sha256);
  const binary = path.join(folder, process.platform === 'win32' ? 'impeccable.exe' : 'impeccable');
  let bytes;
  try { bytes = await fs.readFile(binary); }
  catch (error) { if (error.code !== 'ENOENT') throw error; }
  if (!bytes) {
    if (!setup) throw new Error('engine is not set up; run this helper with --setup within the task\'s download/cache authority');
    console.log(`Download pinned Impeccable ${lock.version} to ${binary}`);
    const response = await fetch(asset.url, { signal: AbortSignal.timeout(120_000) });
    if (!response.ok) throw new Error(`engine download failed: HTTP ${response.status}`);
    const chunks = [];
    let total = 0;
    for await (const chunk of response.body) {
      total += chunk.length;
      if (total > asset.bytes) throw new Error('engine download exceeds pinned size');
      chunks.push(chunk);
    }
    bytes = Buffer.concat(chunks);
    if (bytes.length !== asset.bytes || hash(bytes) !== asset.sha256) throw new Error('engine digest or size mismatch; refusing installation');
    await fs.mkdir(folder, { recursive: true });
    const temp = path.join(folder, `.download-${randomUUID()}`);
    try {
      await fs.writeFile(temp, bytes, { flag: 'wx', mode: 0o755 });
      await fs.rename(temp, binary);
    } finally { await fs.rm(temp, { force: true }); }
  }
  if (bytes.length !== asset.bytes || hash(bytes) !== asset.sha256) throw new Error('cached engine differs from its pin; refusing execution');
  if (setup) console.log(`Verified: ${binary}`);
  else {
    const child = spawn(binary, args, { stdio: 'inherit', env: { ...process.env,
      IMPECCABLE_SKILL_DIR: base, IMPECCABLE_SELF: selfCommand } });
    process.on('SIGINT', () => child.kill('SIGINT'));
    process.on('SIGTERM', () => child.kill('SIGTERM'));
    child.on('error', error => { console.error(error.message); process.exitCode = 1; });
    child.on('exit', (code, signal) => { process.exitCode = code ?? (signal === 'SIGINT' ? 130 : 143); });
  }
} catch (error) { console.error(`impeccable: ${error.message}`); process.exitCode = 1; }
