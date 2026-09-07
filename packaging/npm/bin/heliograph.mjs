#!/usr/bin/env node
// Hands over to the binary, fetching it first if this is the first run.
//
// stdio is inherited because `heliograph mcp` speaks JSON-RPC on stdin and
// stdout: anything this wrapper printed there would be read as a malformed
// message and end the session. Progress and errors go to stderr, which the
// protocol ignores.
import { spawn } from 'node:child_process';
import { resolve } from './resolve.mjs';

let bin;
try {
  bin = await resolve();
} catch (err) {
  process.stderr.write(`heliograph: ${err.message}\n`);
  process.stderr.write('  Releases: https://github.com/dbhq-uk/heliograph/releases\n');
  process.exit(1);
}

// spawn rather than spawnSync: a synchronous child does not forward signals,
// so Ctrl-C would kill this wrapper and orphan a running station poll.
const child = spawn(bin, process.argv.slice(2), { stdio: 'inherit' });
for (const sig of ['SIGINT', 'SIGTERM', 'SIGHUP']) {
  process.on(sig, () => child.kill(sig));
}
child.on('exit', (code, signal) => {
  if (signal) process.kill(process.pid, signal);
  else process.exit(code ?? 1);
});
