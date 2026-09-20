import { Select } from '../../components/ui/select';
import { formatDateTime, localDateInput } from '../../lib/time';
import { Fragment, useEffect, useMemo, useState } from 'react';
import { ChevronRight, Download, Search, Users } from 'lucide-react';
import { Button } from '../../components/ui/button';
import type { Node, Stat } from '../../types';

type UserRow = {
  node: string;
  uid: string;
  user: string;
  hours: number;
  cards: number;
  coverage: number;
  details: Stat[];
};
type Result = { users: UserRow[]; incomplete: boolean; from: number; to: number };
const number = (v: number) => v.toLocaleString('zh-CN', { maximumFractionDigits: 2 });

export function UserStats({
  nodes,
  models,
  request,
}: {
  nodes: Node[];
  models: string[];
  request: <T>(path: string) => Promise<T>;
}) {
  const [period, setPeriod] = useState('7'),
    [node, setNode] = useState(''),
    [model, setModel] = useState(''),
    [user, setUser] = useState(''),
    [sort, setSort] = useState('hours');
  const [start, setStart] = useState(() => localDateInput(new Date(Date.now() - 7 * 86400000))),
    [end, setEnd] = useState(() => localDateInput(new Date()));
  const [result, setResult] = useState<Result | null>(null),
    [error, setError] = useState(''),
    [busy, setBusy] = useState(true),
    [expanded, setExpanded] = useState<Set<string>>(new Set());
  // 固定本次查询的时间边界，导出使用同一范围，不在点击时重新计算“最近七天”。
  const query = useMemo(() => {
    const to = period === 'custom' ? new Date(end).getTime() : Date.now();
    const from = period === 'custom' ? new Date(start).getTime() : to - Number(period) * 86400000;
    if (!Number.isFinite(from) || !Number.isFinite(to) || from >= to) return null;
    return new URLSearchParams({
      view: 'users',
      from: String(from),
      to: String(to),
      node,
      model,
      user,
    }).toString();
  }, [period, start, end, node, model, user]);
  useEffect(() => {
    let active = true;
    setBusy(true);
    setError('');
    setResult(null);
    setExpanded(new Set());
    if (!query) {
      setBusy(false);
      setError('结束时间须晚于开始时间');
      return;
    }
    const timer = setTimeout(() => {
      request<Result>('/stats?' + query)
        .then((data) => {
          if (active) setResult(data);
        })
        .catch((e) => {
          if (active) setError(e.message);
        })
        .finally(() => {
          if (active) setBusy(false);
        });
    }, 200);
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, [query, request]);
  const rows = useMemo(
    () =>
      [...(result?.users || [])].sort((a, b) =>
        sort === 'name'
          ? a.user.localeCompare(b.user) || a.node.localeCompare(b.node)
          : b.hours - a.hours || a.user.localeCompare(b.user),
      ),
    [result, sort],
  );
  const nodeName = (id: string) => nodes.find((n) => n.id === id)?.name || id;
  const range = result ? `${formatDateTime(result.from)} — ${formatDateTime(result.to)}` : '';
  return (
    <>
      <div className="filters stats-filters">
        <Select aria-label="统计范围" value={period} onChange={(e) => setPeriod(e.target.value)}>
          <option value="1">最近 24 小时</option>
          <option value="7">最近 7 天</option>
          <option value="30">最近 30 天</option>
          <option value="custom">自定义时间</option>
        </Select>
        <Select aria-label="统计节点" value={node} onChange={(e) => setNode(e.target.value)}>
          <option value="">全部节点</option>
          {nodes.map((n) => (
            <option key={n.id} value={n.id}>
              {n.name}
            </option>
          ))}
        </Select>
        <Select aria-label="统计型号" value={model} onChange={(e) => setModel(e.target.value)}>
          <option value="">全部型号</option>
          {models.map((m) => (
            <option key={m}>{m}</option>
          ))}
        </Select>
        <label className="search">
          <Search size={15} />
          <input
            placeholder="搜索用户或 UID"
            aria-label="统计用户"
            value={user}
            onChange={(e) => setUser(e.target.value)}
          />
        </label>
        <Button
          variant="outline"
          disabled={busy || !result || !query}
          onClick={() => {
            location.href = '/api/v1/export?' + query;
          }}
        >
          <Download size={15} />
          导出
        </Button>
      </div>
      {period === 'custom' && (
        <div className="filters">
          <label className="form-field">
            开始时间
            <input type="datetime-local" value={start} onChange={(e) => setStart(e.target.value)} />
          </label>
          <label className="form-field">
            结束时间
            <input type="datetime-local" value={end} onChange={(e) => setEnd(e.target.value)} />
          </label>
        </div>
      )}
      <div className="section-line">
        <span className="muted tiny">{range}</span>
        <Select aria-label="统计排序" value={sort} onChange={(e) => setSort(e.target.value)}>
          <option value="hours">占用卡时：从高到低</option>
          <option value="name">按用户名</option>
        </Select>
      </div>
      {error && (
        <div className="alert" role="alert">
          {error}
        </div>
      )}

      <div className="table-wrap">
        <table className="user-stats-table">
          <thead>
            <tr>
              <th>用户 / 节点</th>
              <th>占用卡时</th>
              <th>使用卡数</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => {
              const key = row.node + '/' + row.uid,
                open = expanded.has(key);
              return (
                <Fragment key={key}>
                  <tr
                    className="user-stats-row"
                    onClick={() =>
                      setExpanded((current) => {
                        const next = new Set(current);
                        if (next.has(key)) next.delete(key);
                        else next.add(key);
                        return next;
                      })
                    }
                  >
                    <td>
                      <button
                        className="user-stats-toggle"
                        aria-expanded={open}
                        aria-controls={'detail-' + key}
                        onClick={(e) => {
                          e.stopPropagation();
                          setExpanded((current) => {
                            const next = new Set(current);
                            if (next.has(key)) next.delete(key);
                            else next.add(key);
                            return next;
                          });
                        }}
                      >
                        <ChevronRight size={16} className={open ? 'expanded' : ''} />
                        <span>
                          {row.user || row.uid}
                          <small>
                            {nodeName(row.node)} · UID {row.uid}
                          </small>
                        </span>
                      </button>
                    </td>
                    <td>{number(row.hours)}</td>
                    <td>{row.cards}</td>
                  </tr>
                  {open && (
                    <tr id={'detail-' + key}>
                      <td colSpan={3} className="user-stats-detail">
                        <table>
                          <thead>
                            <tr>
                              <th>GPU</th>
                              <th>型号</th>
                              <th>占用卡时</th>
                              <th>归属覆盖率</th>
                            </tr>
                          </thead>
                          <tbody>
                            {[...row.details]
                              .sort((a, b) => {
                                const gpus =
                                  nodes.find((n) => n.id === row.node)?.snapshot.gpus || [];
                                const ai =
                                    gpus.find((g) => g.uuid === a.uuid)?.index ??
                                    Number.MAX_SAFE_INTEGER,
                                  bi =
                                    gpus.find((g) => g.uuid === b.uuid)?.index ??
                                    Number.MAX_SAFE_INTEGER;
                                return (
                                  ai - bi ||
                                  a.uuid.localeCompare(b.uuid, undefined, { numeric: true })
                                );
                              })
                              .map((d) => {
                                const gpu = nodes
                                  .find((n) => n.id === d.node)
                                  ?.snapshot.gpus?.find((g) => g.uuid === d.uuid);
                                return (
                                  <tr key={d.uuid}>
                                    <td>
                                      {gpu ? `GPU ${gpu.index}` : d.uuid}
                                      <small className="mono">{d.uuid}</small>
                                    </td>
                                    <td>{d.model}</td>
                                    <td>{number(d.occupied_hours)}</td>
                                    <td>
                                      {number(d.coverage * 100)}%
                                      <small>
                                        {number(d.occupancy_seconds / 3600)} /{' '}
                                        {number(d.expected_seconds / 3600)} 小时
                                      </small>
                                    </td>
                                  </tr>
                                );
                              })}
                          </tbody>
                        </table>
                      </td>
                    </tr>
                  )}
                </Fragment>
              );
            })}
          </tbody>
        </table>
        {!rows.length && (
          <div className="table-empty">
            <Users size={22} />
            <p>
              {busy
                ? '正在加载…'
                : error
                  ? '统计加载失败'
                  : user || node || model
                    ? '没有匹配的使用记录'
                    : result?.incomplete
                      ? '无法获取用户统计'
                      : '暂无使用记录'}
            </p>
          </div>
        )}
      </div>
    </>
  );
}
