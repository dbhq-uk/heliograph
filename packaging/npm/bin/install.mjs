#!/usr/bin/env node
// Downloads the heliograph binary for this platform from the GitHub release
// that matches this package's version, and verifies it against the published
// SHA256SUMS.
//
// WHY A DOWNLOAD RATHER THAN A BUNDLED BINARY
//
// Six platforms at about 7MB each is a 42MB package for a 7MB tool, and npm
// would serve all six to everybody. The alternative - one package per platform
// with optionalDependencies - is what esbuild does and is genuinely better, but
// it is six more packages to publish and keep in step for a tool whose primary
// distribution is still the GitHub release.
//
// THE CHECKSUM IS NOT OPTIONAL. This fetches an executable over the network and
// puts it on a developer's PATH. Verifying it against the checksums published
// beside it is the least that owes them, and a failure here deletes the file
// rather than leaving something unverified lying around.
import { createWriteStream } from 'node:fs';
import { chmod, mkdir, readFile, rm, stat } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { pipeline } from 'node:stream/promises';
import { Readable } from 'node:stream';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const pkg = JSON.parse(await readFile(join(here, '..', 'package.json'), 'utf8'));
const tag = `v${pkg.version}`;

const PLATFORMS = {
  'darwin-x64': 'heliograph-darwin-amd64',
  'darwin-arm64': 'heliograph-darwin-arm64',
  'linux-x64': 'heliograph-linux-amd64',
  'linux-arm64': 'heliograph-linux-arm64',
  'win32-x64': 'heliograph-windows-amd64.exe',
  'win32-arm64': 'heliograph-windows-arm64.exe',
};

const key = `${process.platform}-${process.arch}`;
const asset = PLATFORMS[key];
if (!asset) {
  console.error(
    `heliograph: no released binary for ${key}.\n` +
    `Supported: ${Object.keys(PLATFORMS).join(', ')}.\n` +
    `You can still build from source: go install github.com/dbhq-uk/heliograph/cmd/heliograph@${tag}`
  );
  process.exit(1);
}

const base = `https://github.com/dbhq-uk/heliograph/releases/download/${tag}`;
const out = join(here, process.platform === 'win32' ? 'heliograph.exe' : 'heliograph');

async function get(url) {
  const res = await fetch(url, { redirect: 'follow' });
  if (!res.ok) throw new Error(`${res.status} ${res.statusText} for ${url}`);
  return res;
}

try {
  // The checksums first: if this fails there is no point downloading 7MB.
  const sums = await (await get(`${base}/SHA256SUMS`)).text();
  const line = sums.split('\n').find((l) => l.trim().endsWith(asset));
  if (!line) throw new Error(`${asset} is not listed in SHA256SUMS for ${tag}`);
  const want = line.trim().split(/\s+/)[0];

  await mkdir(here, { recursive: true });
  await pipeline(Readable.fromWeb((await get(`${base}/${asset}`)).body), createWriteStream(out));

  const got = createHash('sha256').update(await readFile(out)).digest('hex');
  if (got !== want) {
    await rm(out, { force: true });
    throw new Error(
      `checksum mismatch for ${asset}\n  published: ${want}\n  downloaded: ${got}\n` +
      `The file has been deleted. Do not run it.`
    );
  }

  if (process.platform !== 'win32') await chmod(out, 0o755);
  const { size } = await stat(out);
  console.log(`heliograph ${tag}: ${asset} verified (${(size / 1e6).toFixed(1)}MB)`);
} catch (err) {
  console.error(`heliograph: could not install the binary.\n  ${err.message}`);
  console.error(`  Releases: https://github.com/dbhq-uk/heliograph/releases/tag/${tag}`);
  process.exit(1);
}
