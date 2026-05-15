/**
 * Validates that the TS-side `wcag21` / `apca` produce field-identical
 * output to the Go-side `internal/contrast`. The fixture is generated
 * by `internal/contrast/fixture_gen_test.go` (gated by
 * HUETENSION_GEN_PARITY=1) and committed at
 * `web/src/composables/__fixtures__/contrast.json`.
 *
 * Run:
 *
 *   cd web && bun run scripts/check-contrast-parity.ts
 *
 * Exits non-zero on any drift. Wired into vitest in S10 — until then
 * this script is the S7a checkpoint's "within 0.01 ratio" gate.
 * (Both sides round to 2 decimals, so the comparison is byte-exact.)
 */

import { readFileSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

import { fromHex } from '../src/composables/useColor';
import {
  apca,
  wcag21,
  type APCAResult,
  type WCAG21Result,
} from '../src/composables/useContrast';

interface Case {
  fg: string;
  bg: string;
  wcag21: WCAG21Result;
  apca: APCAResult;
}

const __dirname = dirname(fileURLToPath(import.meta.url));
const FIXTURE = resolve(__dirname, '../src/composables/__fixtures__/contrast.json');

const cases = JSON.parse(readFileSync(FIXTURE, 'utf8')) as Case[];

let pass = 0;
let fail = 0;
const failures: string[] = [];

function diff(label: string, want: object, got: object): string[] {
  const errs: string[] = [];
  for (const [k, wv] of Object.entries(want)) {
    const gv = (got as Record<string, unknown>)[k];
    if (gv !== wv) errs.push(`${label}.${k}: want ${JSON.stringify(wv)}, got ${JSON.stringify(gv)}`);
  }
  return errs;
}

for (const c of cases) {
  const fg = fromHex(c.fg);
  const bg = fromHex(c.bg);
  const errs = [
    ...diff('wcag21', c.wcag21, wcag21(fg, bg)),
    // APCA argument order is (text, bg) — fg plays the text role.
    ...diff('apca', c.apca, apca(fg, bg)),
  ];
  if (errs.length === 0) {
    pass++;
  } else {
    fail++;
    failures.push(`  ${c.fg} on ${c.bg}:\n    ${errs.join('\n    ')}`);
  }
}

console.log(`contrast parity: ${pass} pass, ${fail} fail (${cases.length} total)`);
if (fail > 0) {
  console.error('failures:');
  for (const f of failures) console.error(f);
  process.exit(1);
}
