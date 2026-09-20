import type { LoginCopy } from '../../types';
import { useEffect, useState } from 'react';
import { api } from '../../api/client';
import { Button } from '../../components/ui/button';

export function LoginCopySettings() {
  const [copy, setCopy] = useState<LoginCopy | null>(null),
    [error, setError] = useState(''),
    [saved, setSaved] = useState(false),
    [busy, setBusy] = useState(false);
  useEffect(() => {
    api<LoginCopy>('/login-copy')
      .then(setCopy)
      .catch((e) => setError(e.message));
  }, []);
  return (
    <section className="settings-panel">
      <h3>登录页文案</h3>
      <p className="muted">显示在登录页左侧，对所有访客公开。支持换行，留空即可隐藏对应文字。</p>
      {error && (
        <p className="error-text" role="alert">
          {error}
        </p>
      )}
      {copy && (
        <form
          className="login-copy-form"
          onSubmit={async (e) => {
            e.preventDefault();
            setBusy(true);
            setSaved(false);
            setError('');
            try {
              await api('/login-copy', 'PUT', copy);
              setSaved(true);
            } catch (e) {
              setError((e as Error).message);
            } finally {
              setBusy(false);
            }
          }}
        >
          {(
            [
              { key: 'eyebrow', label: '顶部标语', max: 80 },
              { key: 'title', label: '主标题', max: 120 },
              { key: 'description', label: '说明文字', max: 1000 },
              { key: 'caption', label: '底部文字', max: 200 },
            ] as const
          ).map((f) => (
            <label className="form-field" key={f.key}>
              {f.label}
              <textarea
                aria-label={f.label}
                rows={f.key === 'description' ? 4 : 2}
                maxLength={f.max}
                value={copy[f.key]}
                onChange={(e) => {
                  setCopy({ ...copy, [f.key]: e.target.value });
                  setSaved(false);
                }}
              />
            </label>
          ))}
          <div className="settings-actions">
            <Button disabled={busy} type="submit">
              保存登录页文案
            </Button>
            {saved && (
              <span className="muted" role="status">
                已保存
              </span>
            )}
          </div>
        </form>
      )}
    </section>
  );
}
