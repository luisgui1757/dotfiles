import { spawn } from 'node:child_process'
import { realpathSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

// Query the installed host without starting a model turn or reading credentials.
export async function discoverModels(host, timeoutMs = 20_000) {
  if (!['codex', 'claude'].includes(host)) throw new Error('Choose codex or claude.')
  const args = host === 'codex' ? ['app-server', '--stdio'] : [
    '--print', '--input-format', 'stream-json', '--output-format', 'stream-json',
    '--verbose', '--no-session-persistence', '--tools', '', '--no-chrome',
    '--strict-mcp-config', '--mcp-config', '{"mcpServers":{}}',
    '--settings', '{"disableAllHooks":true,"autoMemoryEnabled":false}',
  ]
  const env = host === 'claude'
    ? { ...process.env, DISABLE_ERROR_REPORTING: '1', DISABLE_AUTOUPDATER: '1' }
    : process.env
  const grouped = process.platform !== 'win32'
  const child = spawn(host, args, { env, detached: grouped, stdio: ['pipe', 'pipe', 'ignore'] })
  const stop = signal => {
    if (!grouped || !child.pid) return child.kill(signal)
    try { process.kill(-child.pid, signal) } catch (error) {
      if (error.code !== 'ESRCH') throw error
    }
  }
  let pending, buffer = '', bytes = 0, failure, closed = false
  const reject = message => {
    failure = new Error(message)
    pending?.reject(failure)
  }
  const closing = new Promise(resolve => child.on('close', () => {
    closed = true
    reject('Host exited before discovery completed.')
    resolve()
  }))
  child.on('error', () => reject('Could not start the installed host.'))
  child.stdin.on('error', () => reject('Could not send the discovery request.'))
  child.stdout.setEncoding('utf8')
  child.stdout.on('data', chunk => {
    bytes += Buffer.byteLength(chunk)
    if (bytes > 2_000_000) return reject('Host discovery response is too large.')
    buffer += chunk
    while (buffer.includes('\n')) {
      const index = buffer.indexOf('\n')
      const line = buffer.slice(0, index)
      buffer = buffer.slice(index + 1)
      if (!line.trim()) continue
      let event
      try { event = JSON.parse(line) } catch { return reject('Invalid host discovery response.') }
      if (!event || typeof event !== 'object') return reject('Invalid host discovery response.')
      if (pending?.matches(event)) {
        const current = pending
        pending = undefined
        current.resolve(event)
      }
    }
  })
  const timer = setTimeout(() => reject('Host model discovery timed out.'), timeoutMs)
  const send = value => child.stdin.write(JSON.stringify(value) + '\n')
  const request = (value, matches) => new Promise((resolve, reject) => {
    if (failure) return reject(failure)
    pending = { resolve, reject, matches }
    send(value)
  })
  try {
    let entries = []
    if (host === 'codex') {
      const init = await request({ id: 1, method: 'initialize', params: {
        clientInfo: { name: 'helix_model_discovery', version: '1' },
      } }, event => event.id === 1 && !('method' in event))
      if (init.error || !init.result) throw new Error('Codex initialization failed.')
      send({ method: 'initialized' })
      let cursor = null
      const cursors = new Set()
      do {
        const reply = await request({ id: 2, method: 'model/list', params: {
          limit: 100, includeHidden: false, cursor,
        } }, event => event.id === 2 && !('method' in event))
        if (reply.error || !Array.isArray(reply.result?.data)) throw new Error('Codex model list is unavailable.')
        entries.push(...reply.result.data.filter(model => !model.hidden))
        cursor = reply.result.nextCursor
        if (cursor != null && (typeof cursor !== 'string' || !cursor || cursors.has(cursor))) {
          throw new Error('Invalid Codex model pagination.')
        }
        if (cursor) cursors.add(cursor)
        if (cursors.size > 100) throw new Error('Codex model pagination exceeded its limit.')
      } while (cursor)
    } else {
      const reply = await request({ type: 'control_request', request_id: 'models',
        request: { subtype: 'initialize' },
      }, event => event.type === 'control_response' && event.response?.request_id === 'models')
      if (reply.response.subtype !== 'success' || !Array.isArray(reply.response.response?.models)) {
        throw new Error('Claude model list is unavailable.')
      }
      entries = reply.response.response.models.filter(model => !['default', 'inherit', 'opusplan'].includes(model.value))
    }
    const models = new Map()
    for (const entry of entries) {
      const id = host === 'codex' ? entry.model : entry.resolvedModel
      const efforts = host === 'codex'
        ? entry.supportedReasoningEfforts?.map(value => value.reasoningEffort)
        : entry.supportedEffortLevels
      if (typeof id !== 'string' || !id || typeof entry.displayName !== 'string' ||
          (efforts != null && (!Array.isArray(efforts) || efforts.some(value => typeof value !== 'string' || !value)))) {
        throw new Error('Host returned incomplete model metadata.')
      }
      const model = { id, name: entry.displayName, efforts: efforts ?? null }
      if (models.has(id) && JSON.stringify(models.get(id).efforts) !== JSON.stringify(model.efforts)) {
        throw new Error('Host returned conflicting model capabilities.')
      }
      if (!models.has(id)) models.set(id, model)
    }
    if (!models.size) throw new Error('Host returned no selectable models.')
    return { host, source: 'installed host catalog', accessVerified: false, models: [...models.values()] }
  } finally {
    clearTimeout(timer)
    child.stdin.end()
    stop('SIGTERM')
    let killTimer
    try {
      await Promise.race([closing, new Promise((resolve, reject) => {
        killTimer = setTimeout(() => {
          try {
            if (!closed) stop('SIGKILL')
            child.stdout.destroy()
            child.stdin.destroy()
            resolve()
          } catch (error) { reject(error) }
        }, 2_000)
      })])
    } finally { clearTimeout(killTimer) }
  }
}

if (process.argv[1] && realpathSync(fileURLToPath(import.meta.url)) === realpathSync(process.argv[1])) {
  try {
    console.log(JSON.stringify(await discoverModels(process.argv[2]), null, 2))
  } catch (error) {
    console.error(error.message)
    process.exitCode = 1
  }
}
