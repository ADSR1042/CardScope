import { formatDateTime } from './time';
export const fmt = (v: number | null | undefined, d = 0) =>
  v == null ? '—' : v.toLocaleString('zh-CN', { maximumFractionDigits: d });
export const bytes = (v: number | null | undefined) => {
  if (v == null) return '—';
  if (v === 0) return '0 B';
  const i = Math.min(4, Math.max(0, Math.floor(Math.log(v) / Math.log(1024))));
  return `${fmt(v / 1024 ** i, i > 0 ? 1 : 0)} ${['B', 'KB', 'MB', 'GB', 'TB'][i]}`;
};
export const when = (v: number) => (v ? formatDateTime(v) : '尚未上报');
