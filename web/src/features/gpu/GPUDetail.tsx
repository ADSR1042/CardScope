import { Select } from '../../components/ui/select';
import { formatTime } from '../../lib/time';
import { label } from './status';
import { useEffect, useState } from 'react';
import { Activity } from 'lucide-react';
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { api } from '../../api/client';
import { fmt, bytes, when } from '../../lib/format';
import { Badge } from '../../components/Badge';
import { DialogDescription, DialogTitle } from '../../components/ui/dialog';
import type { GPU, Node } from '../../types';

export function GPUDetail({ g, node }: { g: GPU; node: Node }) {
  const [history, setHistory] = useState<
      { at: number; util: number | null; memory: number | null }[]
    >([]),
    [events, setEvents] = useState<
      { at: number; kind: string; detail: { status?: string; process_status?: string } }[]
    >([]),
    [hours, setHours] = useState('24'),
    [error, setError] = useState('');
  useEffect(() => {
    let active = true;
    const q = new URLSearchParams({
      node: node.id,
      gpu: g.uuid,
      from: String(Date.now() - Number(hours) * 3600000),
    });
    Promise.all([
      api<{ points: typeof history }>('/history?' + q),
      api<typeof events>('/events?' + q),
    ])
      .then(([h, e]) => {
        if (active) {
          setHistory(h.points);
          setEvents(e);
          setError('');
        }
      })
      .catch((e) => {
        if (active) setError(e.message);
      });
    return () => {
      active = false;
    };
  }, [node.id, g.uuid, hours, node.sample_at]);
  return (
    <>
      <div className="eyebrow">
        {node.name} / GPU {g.index}
      </div>
      <DialogTitle className="dialog-title">{g.name}</DialogTitle>
      <DialogDescription className="mono muted">{g.uuid}</DialogDescription>
      <div className="drawer-metrics">
        {[
          ['利用率', `${fmt(g.util)} %`],
          ['显存', `${bytes(g.memory_used)} / ${bytes(g.memory_total)}`],
          ['温度', `${fmt(g.temperature)} °C`],
          ['功耗', `${fmt(g.power)} W`],
        ].map(([k, v]) => (
          <div key={k}>
            <span>{k}</span>
            <strong>{v}</strong>
          </div>
        ))}
      </div>
      <div className="section-line">
        <h3>GPU 利用率</h3>
        <Select value={hours} onChange={(e) => setHours(e.target.value)} aria-label="趋势范围">
          <option value="1">最近 1 小时</option>
          <option value="24">最近 24 小时</option>
          <option value="168">最近 7 天</option>
        </Select>
      </div>
      {error && <p className="error-text">{error}</p>}
      <div className="chart">
        {history.length ? (
          <ResponsiveContainer width="100%" height={210}>
            <AreaChart data={history}>
              <defs>
                <linearGradient id="gpuFill" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="#16a085" stopOpacity={0.2} />
                  <stop offset="100%" stopColor="#16a085" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 5" vertical={false} stroke="var(--border)" />
              <XAxis
                dataKey="at"
                tickFormatter={(v) => formatTime(v)}
                tick={{ fontSize: 13 }}
                minTickGap={45}
              />
              <YAxis domain={[0, 100]} tick={{ fontSize: 13 }} width={36} />
              <Tooltip
                labelFormatter={(v) => when(Number(v))}
                contentStyle={{
                  background: 'var(--panel)',
                  border: '1px solid var(--border)',
                  borderRadius: 10,
                }}
              />
              <Area
                dataKey="util"
                name="利用率 %"
                stroke="#16a085"
                strokeWidth={2}
                fill="url(#gpuFill)"
                connectNulls={false}
                isAnimationActive={false}
              />
            </AreaChart>
          </ResponsiveContainer>
        ) : (
          <div className="chart-empty">
            <Activity size={22} />
            <span>等待相邻的有效采样生成趋势</span>
          </div>
        )}
      </div>
      <h3 className="detail-heading">
        计算进程 <Badge>{g.processes?.length || 0}</Badge>
      </h3>
      {g.process_status !== 'ok' && (
        <div className="notice">
          {label(g.process_status)}。进程缺失不代表无人使用，此卡的用户占用统计可能不可用。
        </div>
      )}
      <div className="process-list">
        {(g.processes || []).map((p) => (
          <div className="process-item" key={`${p.pid}/${p.created}`}>
            <div>
              <strong>{p.user || '未知'}</strong>
              <small>
                {p.name || '名称不可读'} · PID {p.pid}
              </small>
            </div>
            <div>
              {bytes(p.memory)}
              <small>首次观测 {when(p.first_seen)}</small>
            </div>
          </div>
        ))}
        {!g.processes?.length && <p className="muted">未观测到计算进程</p>}
      </div>
      <h3 className="detail-heading">采集能力</h3>
      <div className="capabilities">
        {Object.entries(g.fields || {}).map(([k, v]) => (
          <div key={k}>
            <span>{k}</span>
            <Badge tone={v === 'ok' ? 'green' : 'neutral'}>{label(v)}</Badge>
          </div>
        ))}
      </div>
      <h3 className="detail-heading">最近事件</h3>
      {events.slice(0, 12).map((e, i) => (
        <div className="event" key={`${e.at}/${i}`}>
          <span className={'event-dot' + (e.kind === 'started' ? '' : ' failed')} />
          <div>
            {e.kind === 'started' ? '开始采集' : '采集失败'}
            <small>{when(e.at)}</small>
          </div>
        </div>
      ))}
      <p className="muted tiny">
        最后观测 {when(g.seen_at)} · 数据源 {g.source || '—'}
      </p>
    </>
  );
}
