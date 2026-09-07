#!/usr/bin/env node
// postinstall: fetch the binary now, so the first run is instant.
//
// PURELY A WARM-UP. It is not where the download has to happen, and it must
// never be: install scripts are disabled in many environments and npm is moving
// toward blocking them by default. bin/heliograph.mjs fetches on first use
// instead, and this only saves that wait.
//
// So it exits 0 whatever happens. A failed warm-up is not a failed install, and
// making it one would break `npm install` on a machine that is merely offline
// at the wrong moment.
import { resolve } from './resolve.mjs';

try {
  const bin = await resolve();
  process.stdout.write(`heliograph: ready at ${bin}\n`);
} catch (err) {
  process.stdout.write(
    `heliograph: could not fetch the binary now (${err.message.split('\n')[0]}).\n` +
    `  This is not fatal. It will be fetched on first use.\n`
  );
}
