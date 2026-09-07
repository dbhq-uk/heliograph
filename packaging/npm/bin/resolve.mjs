// Finds the heliograph binary, downloading it if it is not there yet.
//
// WHY NOT IN postinstall, WHICH IS WHERE THIS STARTED
//
// It was, and CI proved that wrong on the first release that used it: the job
// installed the package and then reported "the binary is missing". npm did not
// run the postinstall.
//
// That is not a CI quirk. Install scripts are disabled by default in many
// environments - `npm ci --ignore-scripts`, security-conscious org configs,
// several CI images - and npm itself is moving toward blocking them by default,
// which is what the `install-scripts not yet covered by allowScripts` warning
// is announcing. A package that only works when scripts are allowed is a
// package that quietly breaks for the people most careful about supply chains.
//
// So the download happens on FIRST USE instead. It runs under the user's own
// invocation rather than implicitly at install time, which is both more
// reliable and easier to defend: nothing fetched an executable behind your back
// while you were installing something else.
//
// The postinstall is kept as a warm-up. When scripts are allowed the download
// happens then and first use is instant; when they are not, this covers it.
import { createWriteStream } from 'node:fs';
import { chmod, mkdir, readFile, rename, rm, stat } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { pipeline } from 'node:stream/promises';
import { Readable } from 'node:stream';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { tmpdir } from 'node:os';

const here = dirname(fileURLToPath(import.meta.url));

export const PLATFORMS = {
  'darwin-x64': 'heliograph-darwin-amd64',
  'darwin-arm64': 'heliograph-darwin-arm64',
  'linux-x64': 'heliograph-linux-amd64',
  'linux-arm64': 'heliograph-linux-arm64',
  'win32-x64': 'heliograph-windows-amd64.exe',
  'win32-arm64': 'heliograph-windows-arm64.exe',
};

const exe = process.platform === 'win32' ? '.exe' : '';

async function exists(p) {
  try { await stat(p); return true; } catch { return false; }
}

async function get(url) {
  const res = await fetch(url, { redirect: 'follow' });
  if (!res.ok) throw new Error(`${res.status} ${res.statusText} for ${url}`);
  return res;
}

// download fetches the asset and verifies it before it is ever runnable.
//
// It writes to a temporary name and renames only after the checksum matches, so
// a failed or interrupted download cannot leave a partial file that a later run
// would find and execute.
async function download(dest, tag, asset) {
  const base = `https://github.com/dbhq-uk/heliograph/releases/download/${tag}`;

  const sums = await (await get(`${base}/SHA256SUMS`)).text();
  const line = sums.split('\n').find((l) => l.trim().endsWith(asset));
  if (!line) throw new Error(`${asset} is not listed in SHA256SUMS for ${tag}`);
  const want = line.trim().split(/\s+/)[0];

  await mkdir(dirname(dest), { recursive: true });
  const tmp = join(dirname(dest), `.heliograph-${process.pid}.part`);
  try {
    await pipeline(Readable.fromWeb((await get(`${base}/${asset}`)).body), createWriteStream(tmp));
    const got = createHash('sha256').update(await readFile(tmp)).digest('hex');
    if (got !== want) {
      throw new Error(
        `checksum mismatch for ${asset}\n  published:  ${want}\n  downloaded: ${got}\n` +
        `Nothing has been installed. Do not run anything from that download.`
      );
    }
    if (process.platform !== 'win32') await chmod(tmp, 0o755);
    await rename(tmp, dest);
  } finally {
    await rm(tmp, { force: true }).catch(() => {});
  }
  return dest;
}

// resolve returns a path to a runnable binary, fetching it if needed.
export async function resolve({ quiet = false } = {}) {
  const pkg = JSON.parse(await readFile(join(here, '..', 'package.json'), 'utf8'));
  const tag = `v${pkg.version}`;
  const key = `${process.platform}-${process.arch}`;
  const asset = PLATFORMS[key];
  if (!asset) {
    throw new Error(
      `no released binary for ${key}.\n` +
      `Supported: ${Object.keys(PLATFORMS).join(', ')}.\n` +
      `From source: go install github.com/dbhq-uk/heliograph/cmd/heliograph@${tag}`
    );
  }

  const inPackage = join(here, `heliograph${exe}`);
  if (await exists(inPackage)) return inPackage;

  // node_modules is not always writable - a read-only install, a container
  // layer, a shared prefix. Fall back to a per-user cache rather than failing.
  const cached = join(tmpdir(), `heliograph-${pkg.version}`, `heliograph${exe}`);
  if (await exists(cached)) return cached;

  for (const dest of [inPackage, cached]) {
    try {
      if (!quiet) process.stderr.write(`heliograph: fetching ${asset} ${tag}\n`);
      return await download(dest, tag, asset);
    } catch (err) {
      // A checksum failure is not a place to try somewhere else. It means the
      // bytes were wrong, and the next location would be wrong the same way.
      if (String(err.message).includes('checksum mismatch')) throw err;
      if (dest === cached) throw err;
    }
  }
}
