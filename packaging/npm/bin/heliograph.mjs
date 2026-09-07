#!/usr/bin/env node
// Hands straight over to the binary. stdio is inherited because `heliograph
// mcp` speaks JSON-RPC on stdin and stdout, and anything this wrapper printed
// would be read as a malformed message and end the session.
import { spawnSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const bin = join(here, process.platform === 'win32' ? 'heliograph.exe' : 'heliograph');
if (!existsSync(bin)) {
  console.error('heliograph: the binary is missing. Reinstall the package, or download it from');
  console.error('  https://github.com/dbhq-uk/heliograph/releases');
  process.exit(1);
}
const r = spawnSync(bin, process.argv.slice(2), { stdio: 'inherit' });
process.exit(r.status === null ? 1 : r.status);
