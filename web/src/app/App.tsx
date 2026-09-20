import { Select } from '../components/ui/select';
import { BrandMark } from '../components/BrandMark';
import { formatTime } from '../lib/time';
import { useEffect, useMemo, useState } from 'react';
import {
  Activity,
  ArrowUpRight,
  ChartNoAxesCombined,
  ChevronRight,
  Clock3,
  LayoutGrid,
  LogOut,
  Microchip,
  Moon,
  Plus,
  Search,
  Server,
  Settings2,
  ShieldCheck,
  Sun,
  X,
} from 'lucide-react';
import { api } from '../api/client';
import { users, idle, bad, capacityKey, capacityLabel } from '../features/gpu/status';
import { Login } from '../features/auth/Login';
import { NodeCard } from '../features/nodes/NodeCard';
import { GPUDetail } from '../features/gpu/GPUDetail';
import { Enrollment } from '../features/nodes/Enrollment';
import { Settings } from '../features/settings/Settings';
import { Button } from '../components/ui/button';
import { UserStats } from '../features/statistics/UserStats';
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '../components/ui/dialog';
import { useSession } from '../features/auth/useSession';
import { useNodes } from '../features/nodes/useNodes';
import { Dashboard } from '../features/dashboard/Dashboard';
import { HeaderClock } from '../components/HeaderClock';

export function App() {
  const { user, ready, signIn, signOut } = useSession();
  const { nodes, updated, error, setError, refresh } = useNodes(user);
  const [page, setPage] = useState('dashboard'),
    [dark, setDark] = useState(localStorage.getItem('gpu-theme') === 'dark'),
    [selected, setSelected] = useState<{ node: string; gpu: string } | null>(null),
    [add, setAdd] = useState(false),
    [code, setCode] = useState(''),
    [pending, setPending] = useState(false);
  const [search, setSearch] = useState(''),
    [status, setStatus] = useState('all'),
    [sort, setSort] = useState('name'),
    [free, setFree] = useState('0'),
    [model, setModel] = useState(''),
    [capacity, setCapacity] = useState(''),
    [tag, setTag] = useState(''),
    [nodeId, setNodeId] = useState('');
  useEffect(() => {
    document.documentElement.classList.toggle('dark', dark);
    localStorage.setItem('gpu-theme', dark ? 'dark' : 'light');
  }, [dark]);
  const all = nodes.flatMap((n) => (n.snapshot.gpus || []).map((g) => ({ g, n })));
  const online = nodes.filter((n) => n.online).length;
  const available = all.filter(({ g, n }) => idle(g, n)).length;
  const faults = all.filter(({ g, n }) => n.online && bad(g)).length;
  const missing = nodes.reduce(
    (sum, n) => sum + (n.online ? Math.max(0, n.expected - (n.snapshot.gpus || []).length) : 0),
    0,
  );
  const models = [...new Set(all.map(({ g }) => g.name))].sort();
  const tags = [...new Set(nodes.flatMap((n) => n.tags || []))].sort((a, b) => a.localeCompare(b));
  const capacities = [...new Set(all.map(({ g }) => capacityKey(g)))].sort((a, b) =>
    a === 'unknown' ? 1 : b === 'unknown' ? -1 : Number(a) - Number(b),
  );
  const filtered = useMemo(
    () =>
      nodes
        .filter((n) => !nodeId || n.id === nodeId)
        .filter((n) => !tag || n.tags?.includes(tag))
        .map((n) => ({
          ...n,
          snapshot: {
            ...n.snapshot,
            gpus: (n.snapshot.gpus || []).filter((g) => {
              const q = search.toLowerCase();
              return (
                (!q ||
                  `${n.name} ${n.snapshot.hostname} ${(n.tags || []).join(' ')} ${g.name} ${users(g).join(' ')}`
                    .toLowerCase()
                    .includes(q)) &&
                (!model || g.name === model) &&
                (!capacity || capacityKey(g) === capacity) &&
                (status === 'all' ||
                  (status === 'idle' && idle(g, n)) ||
                  (status === 'fault' && bad(g)) ||
                  (status === 'offline' && !n.online)) &&
                (Number(free) === 0 ||
                  (g.memory_total != null &&
                    g.memory_used != null &&
                    (g.memory_total - g.memory_used) / 1024 ** 3 >= Number(free)))
              );
            }),
          },
        }))
        .filter(
          (n) =>
            n.snapshot.gpus!.length ||
            ((!search ||
              `${n.name} ${n.snapshot.hostname} ${(n.tags || []).join(' ')}`
                .toLowerCase()
                .includes(search.toLowerCase())) &&
              status === 'all' &&
              !model &&
              !capacity &&
              Number(free) === 0),
        )
        .sort((a, b) =>
          sort === 'idle'
            ? b.snapshot.gpus!.filter((g) => idle(g, b)).length -
              a.snapshot.gpus!.filter((g) => idle(g, a)).length
            : sort === 'fault'
              ? b.snapshot.gpus!.filter(bad).length - a.snapshot.gpus!.filter(bad).length
              : a.name.localeCompare(b.name),
        ),
    [nodes, search, model, capacity, status, free, sort, tag, nodeId],
  );
  const selectedNode = nodes.find((n) => n.id === selected?.node),
    selectedGPU = selectedNode?.snapshot.gpus?.find((g) => g.uuid === selected?.gpu);
  const [createError, setCreateError] = useState('');
  const perform = async (
    fn: () => Promise<unknown>,
    refreshAfter = true,
    onError?: (message: string) => void,
  ) => {
    setPending(true);
    try {
      await fn();
      if (refreshAfter) await refresh();
    } catch (e) {
      if (onError) onError((e as Error).message);
      else setError((e as Error).message);
    } finally {
      setPending(false);
    }
  };
  if (!ready)
    return (
      <div className="loading">
        <Activity className="pulse" />
        正在连接面板…
      </div>
    );
  if (!user) return <Login onLogin={signIn} />;
  return (
    <div className={`app-shell${page === 'dashboard' ? ' dashboard-shell' : ''}`}>
      <aside className="sidebar">
        <a className="brand" href="#" onClick={() => setPage('dashboard')}>
          <BrandMark />
          <span>
            CardScope<small>GPU RESOURCE MONITOR</small>
          </span>
        </a>
        <nav>
          {[
            ['dashboard', '集群概览', LayoutGrid],
            ['overview', '资源查询', Search],
            ['stats', '使用统计', ChartNoAxesCombined],
            ...(user.role === 'admin' ? [['settings', '节点管理', Settings2]] : []),
          ].map(([id, title, Icon]) => {
            const I = Icon as typeof LayoutGrid;
            return (
              <button
                key={id as string}
                aria-label={title as string}
                title={title as string}
                className={page === id ? 'active' : ''}
                onClick={() => setPage(id as string)}
              >
                <I size={18} />
                {title as string}
                {id === 'overview' && <span className="nav-count">{nodes.length}</span>}
              </button>
            );
          })}
        </nav>
        <div className="sidebar-bottom">
          <div className="account">
            <span className="avatar">{user.role === 'admin' ? 'A' : 'V'}</span>
            <div>
              {user.role === 'admin' ? '管理员' : '团队访客'}
              <small>{user.username}</small>
            </div>
            <button title="退出登录" aria-label="退出登录" onClick={() => perform(signOut, false)}>
              <LogOut size={16} />
            </button>
          </div>
        </div>
      </aside>
      <main className="main">
        <header className="topbar">
          <div className="breadcrumb">
            {page !== 'dashboard' && (
              <>
                <span className="breadcrumb-prefix">
                  监控面板 <ChevronRight size={14} />
                </span>
                <strong>
                  {page === 'overview' ? '资源查询' : page === 'stats' ? '使用统计' : '节点管理'}
                </strong>
              </>
            )}
          </div>
          <div className="topbar-right">
            <HeaderClock />
            <button
              className="icon-button"
              onClick={() => setDark(!dark)}
              aria-label={dark ? '切换浅色' : '切换深色'}
            >
              {dark ? <Sun size={18} /> : <Moon size={18} />}
            </button>
          </div>
        </header>
        <div className="page">
          <div className="page-heading">
            <div>
              <h1>
                {page === 'dashboard'
                  ? '集群概览'
                  : page === 'overview'
                    ? '资源查询'
                    : page === 'stats'
                      ? '使用统计'
                      : '连接你的计算节点。'}
              </h1>
              <p>
                {page === 'dashboard'
                  ? '快速了解集群状态，查找可用 GPU，掌握运行健康情况。'
                  : page === 'overview'
                    ? '节点状态与 GPU 使用情况'
                    : page === 'stats'
                      ? '查看用户的 GPU 占用卡时。'
                      : '一次接入，持续上报。所有采集均以普通用户权限运行。'}
              </p>
            </div>
            {user.role === 'admin' && page !== 'dashboard' && (
              <Button
                onClick={() => {
                  setCode('');
                  setAdd(true);
                }}
              >
                <Plus size={16} />
                添加节点
              </Button>
            )}
          </div>
          {error && (
            <div className="alert">
              <Activity size={16} />
              {error}
              <button aria-label="关闭错误" onClick={() => setError('')}>
                <X size={16} />
              </button>
            </div>
          )}
          {page === 'dashboard' && (
            <Dashboard
              nodes={nodes}
              updated={updated}
              error={error}
              onGPU={(n, g) => setSelected({ node: n.id, gpu: g.uuid })}
              onQuery={(options = {}) => {
                setSearch(options.search || '');
                setNodeId(options.nodeId || '');
                setStatus(options.status || 'all');
                setModel(options.model || '');
                setCapacity(options.capacity || '');
                setTag('');
                setFree('0');
                setSort('name');
                setPage('overview');
              }}
            />
          )}
          {page === 'overview' && (
            <>
              <div className="summary-grid">
                {[
                  {
                    title: '在线节点',
                    value: online,
                    unit: `/ ${nodes.length}`,
                    icon: Server,
                    note: '在线 / 总数',
                    tone: 'mint',
                  },
                  {
                    title: 'GPU 总数',
                    value: all.length,
                    unit: '张',
                    icon: Microchip,
                    note: '',
                    tone: 'blue',
                  },
                  {
                    title: '空闲 GPU',
                    value: available,
                    unit: '张',
                    icon: Activity,
                    note: '无计算进程 · 低占用',
                    tone: 'mint',
                  },
                  {
                    title: '异常 GPU',
                    value: faults + missing,
                    unit: '张',
                    icon: ShieldCheck,
                    note: missing ? '含未检测到的显卡' : '采集失败或设备异常',
                    tone: 'amber',
                  },
                ].map((v) => (
                  <div className={'summary ' + v.tone} key={v.title}>
                    <div className="summary-label">
                      {v.title}
                      <v.icon size={18} />
                    </div>
                    <div className="summary-value">
                      {v.value}
                      <span>{v.unit}</span>
                    </div>
                    <div className="summary-note">{v.note}</div>
                  </div>
                ))}
              </div>
              <div className="section-line">
                <h2>
                  计算节点 <span>{nodes.length}</span>
                </h2>
                <span className="muted tiny">
                  <Clock3 size={13} />{' '}
                  {updated ? `更新于 ${formatTime(updated, true)}` : '等待上报'} · 每 5 秒
                </span>
              </div>
              <div className="filters">
                {nodeId && (
                  <Button variant="outline" size="sm" onClick={() => setNodeId('')}>
                    节点：{nodes.find((n) => n.id === nodeId)?.name || nodeId} · 清除定位{' '}
                    <X size={14} />
                  </Button>
                )}
                <Select
                  aria-label="节点标签筛选"
                  value={tag}
                  onChange={(e) => setTag(e.target.value)}
                >
                  <option value="">全部标签</option>
                  {tags.map((value) => (
                    <option key={value} value={value}>
                      {value}
                    </option>
                  ))}
                  {tag && !tags.includes(tag) && <option value={tag}>{tag}（已移除）</option>}
                </Select>
                <label className="search">
                  <Search size={16} />
                  <input
                    aria-label="搜索节点或用户"
                    placeholder="搜索节点、标签、GPU 或使用者…"
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                  />
                </label>
                <Select
                  aria-label="GPU 型号"
                  value={model}
                  onChange={(e) => setModel(e.target.value)}
                >
                  <option value="">全部型号</option>
                  {models.map((m) => (
                    <option key={m}>{m}</option>
                  ))}
                </Select>
                <Select
                  aria-label="显存容量"
                  value={capacity}
                  onChange={(e) => setCapacity(e.target.value)}
                >
                  <option value="">全部显存容量</option>
                  {capacities.map((c) => (
                    <option key={c} value={c}>
                      {capacityLabel(c)}
                    </option>
                  ))}
                </Select>
                <Select
                  aria-label="状态筛选"
                  value={status}
                  onChange={(e) => setStatus(e.target.value)}
                >
                  <option value="all">全部状态</option>
                  <option value="idle">空闲</option>
                  <option value="fault">采集或健康异常</option>
                  <option value="offline">节点离线</option>
                </Select>
                <Select
                  aria-label="剩余显存筛选"
                  value={free}
                  onChange={(e) => setFree(e.target.value)}
                >
                  <option value="0">不限剩余显存</option>
                  {[4, 8, 16, 24, 40, 80].map((v) => (
                    <option key={v} value={v}>
                      剩余 ≥ {v} GB
                    </option>
                  ))}
                </Select>
                <Select aria-label="排序" value={sort} onChange={(e) => setSort(e.target.value)}>
                  <option value="name">按名称排序</option>
                  <option value="idle">空闲优先</option>
                  <option value="fault">异常优先</option>
                </Select>
              </div>
              <div className="node-list">
                {filtered.map((n) => (
                  <NodeCard
                    key={n.id}
                    node={n}
                    fullNode={nodes.find((v) => v.id === n.id)!}
                    onGPU={(g) => setSelected({ node: n.id, gpu: g.uuid })}
                  />
                ))}
                {!filtered.length && (
                  <div className="empty">
                    <Server size={34} />
                    <h3>{nodes.length ? '没有符合条件的 GPU' : '开始连接第一台服务器'}</h3>
                    <p>
                      {nodes.length
                        ? '调整筛选条件，查看其他资源。'
                        : '添加节点后，使用普通用户在服务器运行客户端即可。'}
                    </p>
                    {!nodes.length && user.role === 'admin' && (
                      <Button onClick={() => setAdd(true)}>
                        <Plus size={16} />
                        添加节点
                      </Button>
                    )}
                  </div>
                )}
              </div>
            </>
          )}
          {page === 'stats' && <UserStats nodes={nodes} models={models} request={api} />}
          {page === 'settings' && user.role === 'admin' && (
            <Settings
              nodes={nodes}
              pending={pending}
              perform={perform}
              onCode={(c) => {
                setCode(c);
                setAdd(true);
              }}
            />
          )}
        </div>
      </main>
      <Dialog
        open={add}
        onOpenChange={(v) => {
          setAdd(v);
          setCreateError('');
          if (!v) setCode('');
        }}
      >
        <DialogContent>
          <DialogTitle className="dialog-title">
            {code ? '节点接入已准备好' : '添加计算节点'}
          </DialogTitle>
          <DialogDescription className="muted">
            {code ? '接入码 15 分钟内有效，只能使用一次。' : '为服务器设置一个容易辨认的名称。'}
          </DialogDescription>
          {code ? (
            <Enrollment code={code} />
          ) : (
            <form
              onSubmit={(e) => {
                e.preventDefault();
                const name = new FormData(e.currentTarget).get('name');
                setCreateError('');
                perform(
                  async () => {
                    const d = await api<{ code: string }>('/nodes', 'POST', { name });
                    setCode(d.code);
                  },
                  true,
                  setCreateError,
                );
              }}
            >
              <label className="form-field">
                节点名称
                <input
                  name="name"
                  required
                  maxLength={128}
                  placeholder="例如 training-01"
                  aria-invalid={!!createError}
                  aria-describedby={createError ? 'create-node-error' : undefined}
                  onChange={() => setCreateError('')}
                  autoFocus
                />
              </label>
              {createError && (
                <p id="create-node-error" role="alert" className="field-error">
                  {createError}
                </p>
              )}
              <Button disabled={pending} type="submit">
                生成接入码
                <ArrowUpRight size={16} />
              </Button>
            </form>
          )}
        </DialogContent>
      </Dialog>
      <Dialog
        open={!!selectedGPU}
        onOpenChange={(v) => {
          if (!v) setSelected(null);
        }}
      >
        {selectedGPU && selectedNode && (
          <DialogContent wide>
            <GPUDetail g={selectedGPU} node={selectedNode} />
          </DialogContent>
        )}
      </Dialog>
    </div>
  );
}
