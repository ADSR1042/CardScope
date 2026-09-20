import { BrandMark } from '../../components/BrandMark';
import type { LoginCopy } from '../../types';
import { useEffect, useState } from 'react';
import { ArrowUpRight, ShieldCheck } from 'lucide-react';
import { api } from '../../api/client';
import { Button } from '../../components/ui/button';
import type { User } from '../../types';

export function Login({ onLogin }: { onLogin: (u: User) => void }) {
  const [copy, setCopy] = useState<LoginCopy | null>(null);
  useEffect(() => {
    api<LoginCopy>('/login-copy')
      .then(setCopy)
      .catch(() => {});
  }, []);
  const [error, setError] = useState(''),
    [busy, setBusy] = useState(false);
  return (
    <div className="login-shell">
      <div className="login-art">
        <div className="login-brand">
          <BrandMark />
          <div className="eyebrow">{copy?.eyebrow}</div>
        </div>
        <div className="login-message">
          <h1>{copy?.title}</h1>
          <p>{copy?.description}</p>
        </div>
        <span className="login-caption">{copy?.caption}</span>
      </div>
      <div className="login-panel">
        <div className="login-form">
          <h2>欢迎回来</h2>
          <p className="muted">登录 GPU 监控面板</p>
          <form
            onSubmit={async (e) => {
              e.preventDefault();
              setBusy(true);
              setError('');
              const d = new FormData(e.currentTarget);
              try {
                onLogin(
                  await api<User>('/login', 'POST', {
                    username: d.get('username'),
                    password: d.get('password'),
                  }),
                );
              } catch (e) {
                setError((e as Error).message);
              } finally {
                setBusy(false);
              }
            }}
          >
            <label className="form-field">
              账号
              <input name="username" defaultValue="viewer" autoComplete="username" required />
            </label>
            <label className="form-field">
              密码
              <input name="password" type="password" autoComplete="current-password" required />
            </label>
            {error && (
              <p className="error-text" role="alert">
                {error}
              </p>
            )}
            <Button className="full" disabled={busy}>
              {busy ? '正在登录…' : '进入面板'}
              <ArrowUpRight size={16} />
            </Button>
          </form>
          <p className="login-help">
            <ShieldCheck size={14} />
            团队账号由管理员提供，无需注册。
          </p>
        </div>
      </div>
    </div>
  );
}
