import { formatDateTime, formatTime } from '../../lib/time';
import { useEffect, useMemo, useState } from 'react';
import {
  Activity,
  ArrowRight,
  CheckCircle2,
  CircleHelp,
  Clock3,
  Microchip,
  Search,
  Server,
  TriangleAlert,
  Unplug,
} from 'lucide-react';
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
import { bytes, fmt } from '../../lib/format';
import type { GPU, Node } from '../../types';
import { users, capacityLabel } from '../gpu/status';
import { gpuState, stateLabels, summarize, type GPUState } from './summary';

type Props = {
  nodes: Node[];
  updated: number;
  error: string;
  onQuery: (options?: {
    model?: string;
    capacity?: string;
    status?: string;
    search?: string;
    nodeId?: string;
  }) => void;
  onGPU: (node: Node, gpu: GPU) => void;
};
const time = (value: number) => formatTime(value);

function ClusterTrend({ util, count }: { util: number | null; count: number }) {
  const [points, setPoints] = useState<{ at: number; util: number | null }[]>([]);
  const [error, setError] = useState('');
  const [loaded, setLoaded] = useState(false);
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    let active = true;
    let timer: ReturnType<typeof setTimeout>;
    const refresh = async () => {
      try {
        const data = await api<{ points: typeof points }>('/cluster/history');
        if (active) {
          setPoints(data.points);
          setError('');
        }
      } catch (e) {
        if (active) setError((e as Error).message);
      } finally {
        if (active) {
          setLoaded(true);
          timer = setTimeout(refresh, 60000);
        }
      }
    };
    void refresh();
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, [attempt]);
  return (
    <section className="dash-panel dash-trend" aria-label="集群利用率趋势">
      <h2>最近 24 小时</h2>
      <div className="dash-trend-value">
        <span>GPU 平均利用率</span>
        <strong>{util == null ? '—' : `${fmt(util)}%`}</strong>
      </div>
      {error ? (
        <div className="dash-chart-empty" role="status">
          <TriangleAlert size={22} />
          <span>{error}</span>
          <button className="dash-link" onClick={() => setAttempt((v) => v + 1)}>
            重新加载
          </button>
        </div>
      ) : points.some((p) => p.util != null) ? (
        <div className="dash-chart">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={points} margin={{ top: 8, right: 8, bottom: 0, left: -24 }}>
              <CartesianGrid vertical={false} stroke="var(--border)" />
              <XAxis
                type="number"
                dataKey="at"
                ticks={points.filter((_, index) => index % 24 === 0).map((point) => point.at)}
                domain={['dataMin', 'dataMax']}
                tickFormatter={time}
                minTickGap={40}
                axisLine={false}
                tickLine={false}
                tick={{ fontSize: 11, fill: 'var(--muted)' }}
              />
              <YAxis
                domain={[0, 100]}
                ticks={[0, 50, 100]}
                tickFormatter={(v) => `${v}%`}
                axisLine={false}
                tickLine={false}
                tick={{ fontSize: 11, fill: 'var(--muted)' }}
              />
              <Tooltip
                labelFormatter={(v) => formatDateTime(Number(v))}
                formatter={(v) => [`${Number(v).toFixed(1)}%`, '平均利用率']}
                contentStyle={{
                  background: 'var(--panel)',
                  border: '1px solid var(--border)',
                  borderRadius: 8,
                  color: 'var(--text)',
                  fontSize: 12,
                }}
              />
              <Area
                dataKey="util"
                type="linear"
                stroke="var(--accent)"
                fill="var(--accent)"
                fillOpacity={0.1}
                strokeWidth={2}
                connectNulls={false}
                isAnimationActive={false}
                dot={false}
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      ) : (
        <div className="dash-chart-empty">
          <Activity size={24} />
          <span>{loaded ? '暂无历史数据' : '正在加载趋势…'}</span>
          <small>采集后将逐步形成利用率曲线</small>
        </div>
      )}
      <p className="dash-footnote">当前 {count} 张卡有效采样 · 历史按有效采样时长加权</p>
    </section>
  );
}

function GPUCell({ node, gpu, onClick }: { node: Node; gpu: GPU; onClick: () => void }) {
  const state = gpuState(gpu, node);
  return (
    <div className="dash-gpu-wrap">
      <button
        className={`dash-gpu ${state}`}
        onClick={onClick}
        aria-label={`${node.name} GPU ${gpu.index} · ${gpu.name} · ${stateLabels[state]}`}
        aria-describedby={`tip-${node.id}-${gpu.uuid}`}
      >
        {state === 'attention' ? (
          <TriangleAlert size={12} />
        ) : state === 'unknown' ? (
          <CircleHelp size={12} />
        ) : state === 'offline' ? (
          <Unplug size={12} />
        ) : null}
        <span>{gpu.index}</span>
      </button>
      <div className="dash-gpu-tip" role="tooltip" id={`tip-${node.id}-${gpu.uuid}`}>
        <strong>{gpu.name}</strong>
        <span>
          {node.name} · GPU {gpu.index}
        </span>
        <span>
          {stateLabels[state]} · 利用率 {fmt(gpu.util)}%
        </span>
        <span>
          {node.online ? '剩余显存' : '最后上报剩余显存'}{' '}
          {gpu.memory_total != null && gpu.memory_used != null
            ? bytes(Math.max(0, gpu.memory_total - gpu.memory_used))
            : '—'}
        </span>
        <span>
          {users(gpu).join('、') || (gpu.process_status === 'ok' ? '无计算进程' : '使用者未知')}
        </span>
        <b>
          点击查看详情 <ArrowRight size={12} />
        </b>
      </div>
    </div>
  );
}

export function Dashboard({ nodes, updated, error, onQuery, onGPU }: Props) {
  const data = useMemo(() => summarize(nodes), [nodes]);
  const [allModels, setAllModels] = useState(false);
  const [allIssues, setAllIssues] = useState(false);
  const stale = !!error || (!!updated && Date.now() - updated > 20000);
  const total = data.detected;
  const segments = (Object.keys(stateLabels) as GPUState[]).filter((k) => data.counts[k]);
  return (
    <div className={`dashboard${stale ? ' is-stale' : ''}`}>
      <div className="dash-top-grid">
        <section className="dash-availability">
          <div className="dash-eyebrow">
            <Microchip size={20} /> GPU 当前空闲
          </div>
          <div className="dash-hero-line">
            <div className="dash-big-number">
              {updated ? data.counts.idle : '—'}
              <span>张</span>
            </div>
            <div className="dash-health">
              <Activity size={19} />
              <div>
                <strong>
                  {!updated
                    ? '正在获取状态'
                    : stale
                      ? '数据更新中断'
                      : !nodes.length
                        ? '等待节点接入'
                        : data.issues.length
                          ? `${data.issues.length} 项需要关注`
                          : '集群运行正常'}
                </strong>
                <span>
                  {data.online}/{nodes.length} 节点在线
                </span>
              </div>
            </div>
          </div>
          <p className="dash-hero-caption">
            {total} 张已检测 GPU · {data.online} 个在线节点
            {data.missing > 0 && <span> · 缺失 {data.missing} 张</span>}
          </p>
          <button className="dash-primary" onClick={() => onQuery({ status: 'idle' })}>
            <Search size={17} /> 查看详情 <ArrowRight size={17} />
          </button>
          <div className="dash-capacity">
            <div
              className="dash-capacity-track"
              aria-label={
                segments.map((k) => `${stateLabels[k]} ${data.counts[k]} 张`).join('，') ||
                '暂无 GPU'
              }
            >
              {segments.map((k) => (
                <span
                  key={k}
                  className={k}
                  style={{ width: `${(data.counts[k] / total) * 100}%` }}
                />
              ))}
            </div>
            <div className="dash-capacity-labels">
              {(segments.length ? segments : (['idle', 'busy', 'attention'] as GPUState[])).map(
                (k) => (
                  <div key={k}>
                    <span>
                      <i className={k} />
                      {stateLabels[k]} <strong>{data.counts[k]}</strong>
                    </span>
                    <small>{total ? ((data.counts[k] / total) * 100).toFixed(1) : '0'}%</small>
                  </div>
                ),
              )}
            </div>
          </div>
        </section>
        <section className="dash-panel dash-models">
          <div className="dash-section-heading">
            <h2>型号可用情况</h2>
          </div>
          <div className="dash-model-head">
            <span>GPU 型号</span>
            <span>空闲 / 全部</span>
            <span>显存</span>
          </div>
          {(allModels ? data.models : data.models.slice(0, 3)).map((m) => (
            <button
              key={m.key}
              className="dash-model-row"
              onClick={() => onQuery({ model: m.name, capacity: m.capacity, status: 'idle' })}
              aria-label={`查看 ${m.name} · ${capacityLabel(m.capacity)} 的 ${m.idle} 张空闲 GPU`}
            >
              <strong>{m.name.replace(/^NVIDIA\s+/i, '')}</strong>
              <div>
                <div className="dash-model-bar-line">
                  <div className="dash-model-track">
                    <span style={{ width: `${(m.idle / m.total) * 100}%` }} />
                  </div>
                  <span>
                    {m.idle} / {m.total}
                  </span>
                </div>
                <small>{((m.idle / m.total) * 100).toFixed(1)}% 空闲</small>
              </div>
              <span className="dash-model-memory">{capacityLabel(m.capacity)}</span>
            </button>
          ))}
          {!data.models.length && (
            <div className="dash-inline-empty">
              {updated ? '节点接入后显示型号与可用容量' : '正在获取资源…'}
            </div>
          )}
          {data.models.length > 3 && (
            <button className="dash-link dash-more" onClick={() => setAllModels(!allModels)}>
              {allModels ? '收起型号' : `展开其余 ${data.models.length - 3} 种配置`}
            </button>
          )}
        </section>
      </div>
      <div className="dash-bottom-grid">
        <section className="dash-panel dash-map">
          <div className="dash-section-heading">
            <div>
              <h2>节点负载</h2>
              <p>{nodes.length} 个计算节点 · 按实际 GPU 数量展示</p>
            </div>
          </div>
          <div className="dash-legend">
            {(Object.keys(stateLabels) as GPUState[]).map((k) => (
              <span key={k}>
                <i className={k} />
                {stateLabels[k]}
              </span>
            ))}
          </div>
          <div className="dash-node-grid">
            {[...nodes]
              .sort((a, b) => a.name.localeCompare(b.name, 'zh-CN', { numeric: true }))
              .map((n) => (
                <article className="dash-node" key={n.id}>
                  <div className="dash-node-title">
                    <Server size={15} />
                    <button onClick={() => onQuery({ nodeId: n.id })} title={n.name}>
                      {n.name}
                    </button>
                    <span className={n.online ? 'online' : 'offline'}>
                      {n.online ? '在线' : '离线'}
                    </span>
                  </div>
                  <div className="dash-gpus">
                    {[...(n.snapshot.gpus || [])]
                      .sort((a, b) => a.index - b.index)
                      .map((g) => (
                        <GPUCell key={g.uuid} node={n} gpu={g} onClick={() => onGPU(n, g)} />
                      ))}
                  </div>
                  {!!n.tags?.length && (
                    <div className="node-tags" aria-label="节点标签">
                      {n.tags.map((tag) => (
                        <span className="node-tag" key={tag}>
                          {tag}
                        </span>
                      ))}
                    </div>
                  )}
                  <p className="dash-node-meta">
                    {n.snapshot.gpus?.length || 0} 张 GPU ·{' '}
                    {[
                      ...new Set(
                        (n.snapshot.gpus || []).map((g) => g.name.replace(/^NVIDIA\s+/i, '')),
                      ),
                    ].join(' / ') || '等待采集'}
                  </p>
                  {n.expected > (n.snapshot.gpus?.length || 0) && (
                    <p className="dash-missing">
                      <TriangleAlert size={12} />
                      {n.online ? '未检测到' : '最后记录缺失'}{' '}
                      {n.expected - (n.snapshot.gpus?.length || 0)} 张 · 预期 {n.expected} 张
                    </p>
                  )}
                </article>
              ))}
          </div>
          {!nodes.length && (
            <div className="dash-map-empty">
              <Server size={32} />
              <h3>{updated ? '连接第一台计算节点' : '正在获取节点…'}</h3>
              <p>接入后，这里会显示每个节点的 GPU 状态。</p>
              <button className="dash-link" onClick={() => onQuery()}>
                前往资源查询 <ArrowRight size={14} />
              </button>
            </div>
          )}
        </section>
        <div className="dash-side">
          <ClusterTrend util={data.util} count={data.utilCount} />
          <section className="dash-panel dash-issues">
            <div className="dash-section-heading">
              <h2>
                需要关注 <span className="dash-issue-count">{data.issues.length}</span>
              </h2>
              {data.issues.length > 2 && (
                <button className="dash-link" onClick={() => setAllIssues(!allIssues)}>
                  {allIssues ? '收起' : '查看全部'}
                </button>
              )}
            </div>
            {(allIssues ? data.issues : data.issues.slice(0, 2)).map((issue) => (
              <button
                className="dash-issue"
                key={issue.key}
                onClick={() =>
                  issue.gpu ? onGPU(issue.node, issue.gpu) : onQuery({ nodeId: issue.node.id })
                }
              >
                <TriangleAlert size={19} />
                <span>
                  <strong>{issue.title}</strong>
                  <small>{issue.detail}</small>
                </span>
                <ArrowRight size={13} />
              </button>
            ))}
            {!data.issues.length && (
              <div className="dash-all-good">
                <CheckCircle2 size={22} />
                <span>
                  {!updated
                    ? '正在检查节点状态'
                    : stale
                      ? '等待恢复数据更新'
                      : nodes.length
                        ? '当前没有需要关注的异常'
                        : '等待节点接入'}
                </span>
              </div>
            )}
          </section>
        </div>
      </div>
      <div className="dash-update">
        <Clock3 size={12} />
        {updated
          ? `${stale ? '最后成功更新' : '更新于'} ${time(updated)} · 每 5 秒刷新`
          : '等待首次数据'}
        <span>空闲判定：无计算进程，利用率与显存占用均低于 5%</span>
      </div>
    </div>
  );
}
