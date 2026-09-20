import test from 'node:test';
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import ts from 'typescript';

const source = readFileSync(new URL('../src/lib/time.ts', import.meta.url), 'utf8');
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ESNext },
}).outputText;
const url = `data:text/javascript;base64,${Buffer.from(compiled).toString('base64')}`;
for (const [zone, instant, clock, offset] of [
  ['Asia/Shanghai', '2026-07-01T12:00:00Z', '20:00:00', 'UTC+08:00'],
  ['America/New_York', '2026-07-01T12:00:00Z', '08:00:00', 'UTC-04:00'],
  ['America/New_York', '2026-01-01T12:00:00Z', '07:00:00', 'UTC-05:00'],
  ['Asia/Kathmandu', '2026-07-01T12:00:00Z', '17:45:00', 'UTC+05:45'],
]) {
  test(`${zone} ${instant}: display, offset and local query round trip`, () => {
    const script = `import { formatDateTime, formatTime, localDateInput, timeZoneLabel } from ${JSON.stringify(url)};
      const date = new Date(${JSON.stringify(instant)});
      console.log(JSON.stringify({ full: formatDateTime(date), time: formatTime(date.getTime(), true), zone: timeZoneLabel(date), roundTrip: new Date(localDateInput(date)).getTime() }));`;
    const result = JSON.parse(
      execFileSync(process.execPath, ['--input-type=module', '-e', script], {
        env: { ...process.env, TZ: zone },
        windowsHide: true,
        encoding: 'utf8',
      }),
    );
    assert.equal(result.time, clock);
    assert.ok(result.full.endsWith(clock));
    assert.ok(result.zone.endsWith(offset));
    assert.equal(result.roundTrip, Date.parse(instant));
  });
}
