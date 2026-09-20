import type { GPU, Node } from '../../types';

// Bucket observed total VRAM to whole GiB; small driver reservations must not
// split otherwise identical cards. Unknown capacity remains its own group.
export const capacityKey = (g: GPU) =>
  g.memory_total != null && Number.isFinite(g.memory_total) && g.memory_total > 0
    ? String(Math.max(1, Math.round(g.memory_total / 1024 ** 3)))
    : 'unknown';
export const capacityLabel = (key: string) => (key === 'unknown' ? '显存未知' : `${key} GB`);

export const users = (g: GPU) => [...new Set((g.processes || []).map((p) => p.user || '未知'))];
export const idle = (g: GPU, n: Node) =>
  n.online &&
  g.status === 'ok' &&
  !g.fields?.health &&
  (g.temperature == null || g.temperature < 85) &&
  g.process_status === 'ok' &&
  !(g.processes || []).length &&
  g.util != null &&
  g.util < 5 &&
  g.memory_used != null &&
  g.memory_total != null &&
  g.memory_total > 0 &&
  g.memory_used / g.memory_total < 0.05;
export const bad = (g: GPU) => g.status !== 'ok' || !!g.fields?.health;
const labels: Record<string, string> = {
  ok: '正常',
  unsupported: '不支持',
  permission_denied: '权限不足',
  query_failed: '采集失败',
  timeout: '查询超时',
  not_detected: '未检测到',
  gpu_lost: '驱动报告丢失',
  unknown: '未知',
  wsl_limited: 'WSL 进程信息受限',
  mig_unsupported: 'MIG 归属暂不支持',
  mps_unsupported: 'MPS 归属暂不支持',
  partial: '用户归属不完整',
};
export const label = (s: string) => labels[s] || s || '未知';
