#!/usr/bin/env node

import { lstat, mkdir, readFile, rename, rm, rmdir, writeFile } from "node:fs/promises";
import { randomUUID } from "node:crypto";
import path from "node:path";
import process from "node:process";

const LOCK_RETRIES = 10;
const LOCK_RETRY_MS = 20;

function usage() {
  console.error(
    "usage: configure-pi-theme.mjs set <settings.json> <default-theme> [managed-theme...] | unset <settings.json> <managed-theme> [managed-theme...]",
  );
}

function sleep(milliseconds) {
  return new Promise((resolve) => setTimeout(resolve, milliseconds));
}

async function acquireLock(settingsPath) {
  const lockPath = `${settingsPath}.lock`;
  for (let attempt = 0; attempt <= LOCK_RETRIES; attempt += 1) {
    try {
      await mkdir(lockPath);
      return async () => rmdir(lockPath);
    } catch (error) {
      if (error.code !== "EEXIST") {
        throw error;
      }
      if (attempt === LOCK_RETRIES) {
        throw new Error(`Pi settings are busy: ${lockPath}. Close Pi and retry.`);
      }
      await sleep(LOCK_RETRY_MS);
    }
  }
  throw new Error(`could not acquire Pi settings lock: ${lockPath}`);
}

async function readSettings(settingsPath) {
  try {
    const metadata = await lstat(settingsPath);
    if (!metadata.isFile()) {
      throw new Error(`Pi settings path is not a regular file: ${settingsPath}`);
    }
    const raw = await readFile(settingsPath, "utf8");
    let settings;
    try {
      settings = JSON.parse(raw);
    } catch (error) {
      throw new Error(`Pi settings are not valid JSON: ${settingsPath}: ${error.message}`);
    }
    if (settings === null || typeof settings !== "object" || Array.isArray(settings)) {
      throw new Error(`Pi settings root must be a JSON object: ${settingsPath}`);
    }
    return settings;
  } catch (error) {
    if (error.code === "ENOENT") {
      return {};
    }
    throw error;
  }
}

async function atomicWrite(settingsPath, settings) {
  const directory = path.dirname(settingsPath);
  const temporaryPath = path.join(
    directory,
    `.${path.basename(settingsPath)}.tmp-${process.pid}-${randomUUID()}`,
  );
  try {
    await writeFile(temporaryPath, `${JSON.stringify(settings, null, 2)}\n`, {
      encoding: "utf8",
      flag: "wx",
      mode: 0o600,
    });
    await rename(temporaryPath, settingsPath);
  } catch (error) {
    try {
      await rm(temporaryPath, { force: true });
    } catch (cleanupError) {
      throw new AggregateError([error, cleanupError], `could not publish Pi settings: ${settingsPath}`);
    }
    throw error;
  }
}

async function main() {
  const [command, settingsPath, ...themeNames] = process.argv.slice(2);
  const hasValidThemeCount =
    (command === "set" && themeNames.length > 0) ||
    (command === "unset" && themeNames.length > 0);
  if (!(["set", "unset"].includes(command)) || !settingsPath || !hasValidThemeCount) {
    usage();
    process.exitCode = 2;
    return;
  }
  const themeName = themeNames[0];

  if (command === "unset") {
    try {
      await lstat(settingsPath);
    } catch (error) {
      if (error.code === "ENOENT") {
        console.log("kept user-selected Pi theme: <unset>");
        return;
      }
      throw error;
    }
  }

  await mkdir(path.dirname(settingsPath), { recursive: true, mode: 0o700 });
  const unlock = await acquireLock(settingsPath);
  let operationError;
  try {
    const settings = await readSettings(settingsPath);
    if (command === "set") {
      if (themeNames.includes(settings.theme)) {
        console.log(`unchanged managed Pi theme: ${settings.theme}`);
        return;
      }
      settings.theme = themeName;
      await atomicWrite(settingsPath, settings);
      console.log(`set Pi theme: ${themeName}`);
      return;
    }

    if (!themeNames.includes(settings.theme)) {
      console.log(`kept user-selected Pi theme: ${settings.theme ?? "<unset>"}`);
      return;
    }
    const selectedTheme = settings.theme;
    delete settings.theme;
    await atomicWrite(settingsPath, settings);
    console.log(`removed managed Pi theme: ${selectedTheme}`);
  } catch (error) {
    operationError = error;
    throw error;
  } finally {
    try {
      await unlock();
    } catch (unlockError) {
      if (operationError) {
        throw new AggregateError([operationError, unlockError], `Pi settings update and unlock both failed: ${settingsPath}`);
      }
      throw unlockError;
    }
  }
}

main().catch((error) => {
  console.error(`FAIL: ${error.message}`);
  process.exitCode = 1;
});
