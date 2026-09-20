import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { transformWithOxc } from 'vite';

// Run the production selectors without adding a browser or test-runner dependency.
async function moduleURL(path, replacements = {}) {
  let source = readFileSync(new URL(path, import.meta.url), 'utf8');
  for (const [from, to] of Object.entries(replacements)) source = source.replace(from, to);
  const { code: outputText } = await transformWithOxc(source, path, { target: 'es2022' });
  return `data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`;
}
const statusURL = await moduleURL('../src/features/gpu/status.ts');
const { summarize, gpuState } = await import(
  await moduleURL('../src/features/dashboard/summary.ts', { '../gpu/status': statusURL })
);
const { idle, capacityKey } = await import(statusURL);
const gpu = (changes = {}) => ({
  uuid: 'gpu',
  name: 'GPU model',
  index: 0,
  status: 'ok',
  process_status: 'ok',
  processes: [],
  util: 0,
  memory_used: 0,
  memory_total: 24 * 1024 ** 3,
  temperature: 35,
  fields: {},
  ...changes,
});
const node = (gpus, changes = {}) => ({
  id: 'node',
  name: 'node',
  online: true,
  enrolled: true,
  expected: gpus.length,
  snapshot: { gpus, gpu_status: 'ok' },
  ...changes,
});

test('heterogeneous nodes count actual GPUs and report missing cards separately', () => {
  const nodes = [1, 2, 4, 8, 12].map((count, i) =>
    node(
      Array.from({ length: count }, (_, j) => gpu({ uuid: `${i}-${j}`, index: j })),
      { id: `${i}`, expected: i === 0 ? 2 : count },
    ),
  );
  const result = summarize(nodes);
  assert.equal(result.detected, 27);
  assert.equal(result.counts.idle, 27);
  assert.equal(result.missing, 1);
  assert.equal(result.issues[0].title, '未检测到 1 张 GPU');
});
test('offline, unknown and unhealthy cards are never offered as available', () => {
  for (const card of [
    gpu({ temperature: 90 }),
    gpu({ fields: { health: 'ecc' } }),
    gpu({ status: 'timeout' }),
    gpu({ process_status: 'unknown' }),
    gpu({ util: null }),
  ]) {
    const n = node([card]);
    assert.notEqual(gpuState(card, n), 'idle');
    assert.equal(idle(card, n), false);
  }
  const offline = node([gpu()], { online: false });
  assert.equal(summarize([offline]).counts.offline, 1);
  assert.equal(summarize([offline]).util, null);
});
test('state categories partition detected cards and model counts include mixed models', () => {
  const n = node([
    gpu(),
    gpu({ name: 'Other', processes: [{ user: 'alice' }] }),
    gpu({ status: 'timeout' }),
    gpu({ process_status: 'unknown' }),
  ]);
  const result = summarize([n]);
  assert.equal(
    Object.values(result.counts).reduce((a, b) => a + b, 0),
    result.detected,
  );
  assert.equal(result.models.length, 2);
  assert.equal(result.models[0].idle, 1);
  assert.equal(result.models[0].total, 3);
});
test('empty clusters have no fabricated utilization or capacity', () => {
  const result = summarize([]);
  assert.equal(result.detected, 0);
  assert.equal(result.util, null);
  assert.deepEqual(result.models, []);
  assert.deepEqual(result.issues, []);
});

test('same-name 4090 cards split by total VRAM with shared query keys', () => {
  const cards = [
    gpu({ name: 'NVIDIA RTX 4090', memory_total: 24 * 1024 ** 3 }),
    gpu({ name: 'NVIDIA RTX 4090', memory_total: 24 * 1024 ** 3 - 256 * 1024 ** 2 }),
    gpu({ name: 'NVIDIA RTX 4090', memory_total: 48 * 1024 ** 3 }),
    gpu({
      name: 'NVIDIA RTX 4090',
      memory_total: 48 * 1024 ** 3 - 128 * 1024 ** 2,
      processes: [{ user: 'alice' }],
    }),
    gpu({ name: 'NVIDIA RTX 4090', memory_total: null }),
  ];
  const result = summarize([node(cards)]);
  assert.equal(result.models.length, 3);
  assert.deepEqual(
    result.models.map((m) => [m.capacity, m.total, m.idle]),
    [
      ['24', 2, 2],
      ['48', 2, 1],
      ['unknown', 1, 0],
    ],
  );
  for (const group of result.models) {
    assert.equal(
      cards.filter((g) => g.name === group.name && capacityKey(g) === group.capacity).length,
      group.total,
    );
  }
  assert.equal(new Set(result.models.map((m) => m.key)).size, 3);
});
