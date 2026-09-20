import type { GPU, Node } from '../../types';
import { bad, idle, capacityKey } from '../gpu/status';

export type GPUState = 'idle' | 'busy' | 'attention' | 'unknown' | 'offline';
export const stateLabels: Record<GPUState, string> = {
  idle: '空闲',
  busy: '使用中',
  attention: '异常',
  unknown: '信息不完整',
  offline: '离线',
};
export function gpuState(g: GPU, n: Node): GPUState {
  if (!n.online) return 'offline';
  if (bad(g) || (g.temperature != null && g.temperature >= 85)) return 'attention';
  if (idle(g, n)) return 'idle';
  if (g.process_status !== 'ok' || g.util == null || g.memory_used == null || !g.memory_total)
    return 'unknown';
  return 'busy';
}
export function summarize(nodes: Node[]) {
  const counts: Record<GPUState, number> = {
    idle: 0,
    busy: 0,
    attention: 0,
    unknown: 0,
    offline: 0,
  };
  const models = new Map<
    string,
    { key: string; name: string; capacity: string; total: number; idle: number }
  >();
  const issues: { key: string; title: string; detail: string; node: Node; gpu?: GPU }[] = [];
  let detected = 0,
    missing = 0,
    utilSum = 0,
    utilCount = 0;
  for (const n of nodes) {
    const gpus = n.snapshot.gpus || [];
    if (!n.online)
      issues.push({
        key: n.id,
        title: n.enrolled ? '节点离线' : '等待节点接入',
        detail: n.name,
        node: n,
      });
    else if (n.snapshot.gpu_status && n.snapshot.gpu_status !== 'ok')
      issues.push({ key: `${n.id}-collect`, title: 'GPU 采集受限', detail: n.name, node: n });
    const absent = Math.max(0, n.expected - gpus.length);
    missing += absent;
    if (absent && n.online)
      issues.push({
        key: `${n.id}-missing`,
        title: `未检测到 ${absent} 张 GPU`,
        detail: `${n.name} · 预期 ${n.expected} 张`,
        node: n,
      });
    for (const g of gpus) {
      detected++;
      const state = gpuState(g, n);
      counts[state]++;
      const capacity = capacityKey(g);
      const key = JSON.stringify([g.name, capacity]);
      const m = models.get(key) || {
        key,
        capacity,
        name: g.name,
        total: 0,
        idle: 0,
      };
      m.total++;
      if (state === 'idle') m.idle++;
      models.set(key, m);
      if (n.online && g.status === 'ok' && g.util != null) {
        utilSum += g.util;
        utilCount++;
      }
      if (state === 'attention' || state === 'unknown')
        issues.push({
          key: `${n.id}-${g.uuid}`,
          node: n,
          gpu: g,
          title: bad(g)
            ? '采集或健康异常'
            : state === 'unknown'
              ? '采集信息不完整'
              : `温度偏高 · ${g.temperature}°C`,
          detail: `${n.name} · GPU ${g.index}`,
        });
    }
  }
  return {
    counts,
    detected,
    missing,
    issues,
    models: [...models.values()].sort((a, b) => b.idle - a.idle || a.name.localeCompare(b.name)),
    online: nodes.filter((n) => n.online).length,
    util: utilCount ? utilSum / utilCount : null,
    utilCount,
  };
}
