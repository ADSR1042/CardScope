import { users, idle, bad, label } from '../gpu/status';
import { useState } from 'react';
import {
  ArrowDown,
  ArrowUp,
  ChevronDown,
  ChevronRight,
  Cpu,
  MemoryStick,
  Network,
  Server,
} from 'lucide-react';
import { fmt, bytes, when } from '../../lib/format';
import { Meter } from '../../components/Meter';
import { Badge } from '../../components/Badge';
import { NodeResources } from './NodeResources';
import type { GPU, Node } from '../../types';

export function NodeCard({
  node: n,
  fullNode,
  onGPU,
}: {
  node: Node;
  fullNode: Node;
  onGPU: (g: GPU) => void;
}) {
  const [expanded, setExpanded] = useState(false),
    [collapsed, setCollapsed] = useState(true);
  const cards = fullNode.snapshot.gpus || [];
  const idleCards = cards.filter((g) => idle(g, fullNode)).length;
  const busyCards = cards.filter(
    (g) => fullNode.online && g.status === 'ok' && g.process_status === 'ok' && !idle(g, fullNode),
  ).length;
  const unknownCards = cards.length - idleCards - busyCards;
  const powered = cards.filter((g) => fullNode.online && g.status === 'ok' && g.power != null);
  const totalPower = powered.reduce((sum, g) => sum + g.power!, 0);
  const s = n.snapshot.system;
  const networks = s.networks || [];
  const chosen = networks.filter((i) =>
    n.networks?.length ? n.networks.includes(i.name) : i.primary,
  );
  const rate = (key: 'rx_rate' | 'tx_rate') =>
    chosen.length && chosen.every((i) => i[key] != null)
      ? chosen.reduce((sum, i) => sum + i[key]!, 0)
      : null;
  const mem = s.memory_total
    ? ((s.memory_total - s.memory_available) / s.memory_total) * 100
    : null;
  return (
    <article className={'node-card ' + (!n.online ? 'node-offline' : '')}>
      <div
        className="node-header"
        role="button"
        tabIndex={0}
        aria-label={`${collapsed ? '展开' : '收起'} ${n.name}`}
        aria-expanded={!collapsed}
        aria-controls={'node-body-' + n.id}
        onClick={() => setCollapsed((v) => !v)}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            setCollapsed((v) => !v);
          }
        }}
      >
        <div className="node-identity">
          <span className="node-icon">
            <Server size={21} />
          </span>
          <div>
            <h3>
              {n.name}
              <Badge tone={n.online ? 'green' : 'neutral'}>
                {n.online ? '在线' : n.enrolled ? '离线' : '待接入'}
              </Badge>
              {s.wsl && <Badge>WSL</Badge>}
            </h3>
            <p>
              {n.snapshot.hostname || '等待客户端连接'} <span>·</span>{' '}
              {n.snapshot.gpus?.length || 0} 张 GPU{' '}
              {n.expected > (n.snapshot.gpus?.length || 0) && (
                <span className="error-text"> / 预期 {n.expected} 张</span>
              )}
            </p>
            {!!n.tags?.length && (
              <div className="node-tags" aria-label="节点标签">
                {n.tags.map((tag) => (
                  <span className="node-tag" key={tag}>
                    {tag}
                  </span>
                ))}
              </div>
            )}
          </div>
        </div>
        <div className="node-summary" aria-label="节点 GPU 概况">
          <span>
            空闲 <strong className="idle-count">{idleCards}</strong>
          </span>
          <span>
            占用 <strong>{busyCards}</strong>
          </span>
          {unknownCards > 0 && (
            <span>
              未知 <strong>{unknownCards}</strong>
            </span>
          )}
          <span className="node-power">
            总功耗 <strong>{powered.length ? `${fmt(totalPower)} W` : '—'}</strong>
            {powered.length > 0 && powered.length < cards.length && (
              <small>
                （{powered.length}/{cards.length} 卡）
              </small>
            )}
          </span>
        </div>
        <span className="detail-button node-collapse" aria-hidden="true">
          {collapsed ? '展开' : '收起'}
          <ChevronDown size={15} className={!collapsed ? 'rotate' : ''} />
        </span>
      </div>
      <div id={'node-body-' + n.id} hidden={collapsed}>
        <div className="gpu-table">
          <div className="gpu-table-head">
            <span>GPU / 型号</span>
            <span>利用率</span>
            <span>显存</span>
            <span>使用者</span>
            <span>状态</span>
            <span />
          </div>
          {(n.snapshot.gpus || []).map((g) => {
            const free = idle(g, n);
            const us = users(g);
            return (
              <button className="gpu-row" key={g.uuid} onClick={() => onGPU(g)}>
                <div className="gpu-name">
                  <span className="gpu-index">{String(g.index).padStart(2, '0')}</span>
                  <span>
                    <strong>{g.name.replace('NVIDIA ', '')}</strong>
                    <small>
                      {g.temperature != null ? `${fmt(g.temperature)}°C` : '温度 —'}
                      <span> · </span>
                      {g.power != null ? `${fmt(g.power)} W` : '功耗 —'}
                    </small>
                  </span>
                </div>
                <div>
                  <div className="metric-heading">
                    {fmt(g.util)}
                    <span>%</span>
                  </div>
                  <Meter value={g.util} />
                </div>
                <div>
                  <div className="metric-heading">
                    {g.memory_used == null ? '—' : fmt(g.memory_used / 1024 ** 3, 1)}
                    <span>
                      {' '}
                      / {g.memory_total == null ? '—' : fmt(g.memory_total / 1024 ** 3, 1)} GB
                    </span>
                  </div>
                  <Meter
                    tone="blue"
                    value={
                      g.memory_total && g.memory_used != null
                        ? (g.memory_used / g.memory_total) * 100
                        : null
                    }
                  />
                </div>
                <div className="gpu-users">
                  {us.length ? (
                    <>
                      <span className="user-dot">{us[0].slice(0, 1).toUpperCase()}</span>
                      <span title={us.join(', ')}>
                        {us[0]}
                        {us.length > 1 ? ` +${us.length - 1}` : ''}
                      </span>
                    </>
                  ) : (
                    <span className="muted">
                      {g.process_status === 'ok' ? '无计算进程' : '归属未知'}
                    </span>
                  )}
                </div>
                <div>
                  <Badge
                    tone={
                      !n.online
                        ? 'neutral'
                        : bad(g)
                          ? 'red'
                          : free
                            ? 'green'
                            : g.process_status !== 'ok'
                              ? 'amber'
                              : 'neutral'
                    }
                  >
                    {!n.online
                      ? '数据过期'
                      : bad(g)
                        ? label(g.status === 'ok' ? '健康告警' : g.status)
                        : free
                          ? '空闲'
                          : g.process_status !== 'ok'
                            ? '信息受限'
                            : '使用中'}
                  </Badge>
                </div>
                <ChevronRight size={16} className="muted" />
              </button>
            );
          })}
          {!n.snapshot.gpus?.length && (
            <div className="no-gpu">
              {n.sample_at ? '暂无可读取的 GPU，检查节点采集能力。' : '节点尚未上报数据。'}
            </div>
          )}
        </div>
        <div className="node-footer">
          <span>
            <Cpu size={14} />
            CPU <strong>{fmt(s.cpu)}%</strong>
          </span>
          <span>
            <MemoryStick size={14} />
            内存 <strong>{fmt(mem)}%</strong>
          </span>
          <span>
            <Network size={14} />
            {n.networks?.length ? '选定网卡' : chosen[0]?.name || '网络'} <ArrowDown size={12} />
            <strong>{bytes(rate('rx_rate'))}/s</strong>
            <ArrowUp size={12} />
            <strong>{bytes(rate('tx_rate'))}/s</strong>
          </span>
          <span className="last-report">{n.online ? '5 秒采样' : when(n.sample_at)}</span>
          <button
            className="detail-button system-toggle"
            onClick={() => setExpanded(!expanded)}
            aria-expanded={expanded}
            aria-controls={'node-details-' + n.id}
          >
            {expanded ? '收起详情' : '系统详情'}
            <ChevronDown size={15} className={expanded ? 'rotate' : ''} />
          </button>
        </div>
        {expanded && <NodeResources node={fullNode} />}
      </div>
    </article>
  );
}
