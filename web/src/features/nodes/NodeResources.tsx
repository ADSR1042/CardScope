import { ArrowDown, ArrowUp, Cpu, MemoryStick, HardDrive, Network } from 'lucide-react';
import type { Node, Net } from '../../types';

const byName = (a: string, b: string) =>
  a.localeCompare(b, 'zh-CN', { numeric: true, sensitivity: 'base' });
const number = (v: number) => v.toLocaleString('zh-CN', { maximumFractionDigits: 1 });
const bytes = (v: number | null) => {
  if (v == null) return '—';
  if (v <= 0) return '0 B';
  const i = Math.min(4, Math.floor(Math.log(v) / Math.log(1024)));
  return `${number(v / 1024 ** i)} ${['B', 'KB', 'MB', 'GB', 'TB'][i]}`;
};
const ratio = (used: number, total: number) =>
  total > 0 ? Math.max(0, Math.min(100, (used / total) * 100)) : null;
const status = (s: string) =>
  ({
    ok: '正常',
    timeout: '查询超时',
    permission_denied: '权限不足',
    unsupported: '不支持',
    query_failed: '采集失败',
  })[s] || s;
function Bar({ value, label }: { value: number | null; label: string }) {
  return (
    <div
      className={
        'resource-bar ' +
        (value == null ? 'unavailable' : value >= 90 ? 'high' : value >= 75 ? 'warm' : '')
      }
      role="progressbar"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={value == null ? undefined : Math.round(value)}
      aria-valuetext={value == null ? '不可用' : `${number(value)}%`}
    >
      <span style={{ width: `${value ?? 0}%` }} />
    </div>
  );
}
function Interface({ net }: { net: Net }) {
  return (
    <div className="interface-row">
      <div className="interface-heading">
        <strong>{net.name}</strong>
        {!net.up && <small>未连接</small>}
      </div>
      <div className="interface-rates">
        <span>
          <ArrowUp size={12} />
          {bytes(net.tx_rate)}/s
        </span>
        <span>
          <ArrowDown size={12} />
          {bytes(net.rx_rate)}/s
        </span>
      </div>
      <div className="interface-totals">
        累计上传 {bytes(net.tx)} · 下载 {bytes(net.rx)}
      </div>
    </div>
  );
}
export function NodeResources({ node: n }: { node: Node }) {
  const s = n.snapshot.system,
    errors = s.errors || {};
  const mem = errors.memory ? null : ratio(s.memory_total - s.memory_available, s.memory_total);
  const swap = errors.swap ? null : ratio(s.swap_used, s.swap_total);
  const nets = [...(s.networks || [])].sort((a, b) => byName(a.name, b.name)),
    main = nets.filter((v) => (n.networks?.length ? n.networks.includes(v.name) : v.primary)),
    others = nets.filter((v) => !main.includes(v));
  const rate = (key: 'rx_rate' | 'tx_rate') =>
    main.length && main.every((v) => v[key] != null)
      ? main.reduce((sum, v) => sum + v[key]!, 0)
      : null;
  const metrics = [
    {
      name: 'CPU',
      icon: Cpu,
      value: s.cpu,
      detail: `${s.cores || '—'} 核 · 负载 ${s.load == null ? '—' : number(s.load)}`,
    },
    {
      name: '内存',
      icon: MemoryStick,
      value: mem,
      detail:
        mem == null
          ? '不可用'
          : `${bytes(s.memory_total - s.memory_available)} / ${bytes(s.memory_total)}`,
    },
    {
      name: 'Swap',
      icon: HardDrive,
      value: swap,
      detail: errors.swap
        ? '不可用'
        : s.swap_total
          ? `${bytes(s.swap_used)} / ${bytes(s.swap_total)}`
          : '未启用',
    },
  ];
  return (
    <div className="node-expanded resource-details" id={'node-details-' + n.id}>
      {!n.online && <div className="resource-stale">最后一次采集数据</div>}
      <div className="resource-metrics">
        {metrics.map((m) => (
          <div className="resource-metric" key={m.name}>
            <div className="resource-label">
              <m.icon size={15} />
              {m.name}
            </div>
            <strong className="resource-value">
              {m.value == null ? '—' : number(m.value) + '%'}
            </strong>
            <Bar label={m.name + '使用率'} value={m.value} />
            <small>{m.detail}</small>
          </div>
        ))}
        <div className="resource-metric network-metric">
          <div className="resource-label">
            <Network size={15} />
            {main.length ? main.map((v) => v.name).join(' + ') : '网络'}
          </div>
          <div className="network-speeds">
            <div>
              <span>
                <ArrowUp size={12} />
                上传
              </span>
              <strong>
                {bytes(rate('tx_rate'))}
                <small>/s</small>
              </strong>
            </div>
            <div>
              <span>
                <ArrowDown size={12} />
                下载
              </span>
              <strong>
                {bytes(rate('rx_rate'))}
                <small>/s</small>
              </strong>
            </div>
          </div>
        </div>
      </div>
      <div className="resource-sections">
        <section className="resource-panel">
          <h4>磁盘</h4>
          <div className="disk-grid">
            {[...(s.disks || [])]
              .sort((a, b) => byName(a.path, b.path))
              .map((d) => {
                const used = d.status === 'ok' ? ratio(d.total - d.free, d.total) : null;
                return (
                  <div className="disk-usage" key={d.path}>
                    <div className="disk-heading">
                      <strong title={d.path}>{d.path}</strong>
                      <span>
                        {used == null
                          ? status(d.status === 'ok' ? 'unknown' : d.status)
                          : number(used) + '%'}
                      </span>
                    </div>
                    <Bar label={d.path + ' 磁盘使用率'} value={used} />
                    <small>
                      {used == null
                        ? '容量不可用'
                        : `${bytes(d.total - d.free)} / ${bytes(d.total)} · 可用 ${bytes(d.free)}`}
                    </small>
                  </div>
                );
              })}
          </div>
          {!s.disks?.length && <span className="muted">暂无磁盘数据</span>}
          <details className="resource-disclosure">
            <summary>
              设备 I/O <span>{s.io?.length || 0}</span>
            </summary>
            <div className="io-list resource-scroll" tabIndex={0} aria-label="设备 I/O 列表">
              {[...(s.io || [])]
                .sort((a, b) => byName(a.device, b.device))
                .map((v) => (
                  <div className="io-row" key={v.device}>
                    <strong>{v.device}</strong>
                    <span>读 {bytes(v.read_rate)}/s</span>
                    <span>写 {bytes(v.write_rate)}/s</span>
                  </div>
                ))}
              {!s.io?.length && <span className="muted">暂无 I/O 数据</span>}
            </div>
          </details>
        </section>
        <section className="resource-panel network-panel">
          <h4>网络</h4>
          <div
            className="primary-interfaces resource-scroll"
            tabIndex={0}
            aria-label="主要网卡列表"
          >
            {main.map((v) => (
              <Interface key={v.name} net={v} />
            ))}
          </div>
          {!main.length && <p className="muted">未选择汇总网卡</p>}
          {others.length > 0 && (
            <details className="resource-disclosure">
              <summary>
                其他网卡 <span>{others.length}</span>
              </summary>
              <div
                className="other-interfaces resource-scroll"
                tabIndex={0}
                aria-label="其他网卡列表"
              >
                {others.map((v) => (
                  <Interface key={v.name} net={v} />
                ))}
              </div>
            </details>
          )}
        </section>
      </div>
      <details className="resource-disclosure collection-details">
        <summary>采集信息</summary>
        <div>
          GPU {status(n.snapshot.gpu_status)} · 缓存淘汰 {n.snapshot.cache_dropped || 0} 个样本
          {s.wsl && <p>系统指标来自 WSL，GPU 与 Windows 共享。</p>}
          {Object.entries(errors).map(([key, value]) => (
            <p key={key}>
              {key}：{status(value)}
            </p>
          ))}
        </div>
      </details>
    </div>
  );
}
