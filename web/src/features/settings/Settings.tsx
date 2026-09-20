import { Select } from '../../components/ui/select';
import { ShieldCheck } from 'lucide-react';
import { api } from '../../api/client';
import { NodeSettings } from '../nodes/NodeSettings';
import { LoginCopySettings } from './LoginCopySettings';
import { Button } from '../../components/ui/button';
import type { Node } from '../../types';

export function Settings({
  nodes,
  pending,
  perform,
  onCode,
}: {
  nodes: Node[];
  pending: boolean;
  perform: (
    fn: () => Promise<unknown>,
    refreshAfter?: boolean,
    onError?: (message: string) => void,
  ) => Promise<void>;
  onCode: (code: string) => void;
}) {
  return (
    <>
      <div className="notice">
        <ShieldCheck size={17} />
        <span>节点凭据仅允许上报自身数据。撤销或重新签发会使旧凭据立即失效。</span>
      </div>
      <div className="settings-list">
        {nodes.map((n) => (
          <NodeSettings key={n.id} node={n} pending={pending} perform={perform} onCode={onCode} />
        ))}
        {!nodes.length && <div className="empty">还没有节点，点击右上角添加。</div>}
      </div>
      <LoginCopySettings />
      <section className="settings-panel">
        <h3>账号密码</h3>
        <p className="muted">修改后对应账号的所有会话将失效。</p>
        <form
          className="password-form"
          onSubmit={(e) => {
            e.preventDefault();
            const form = e.currentTarget;
            const d = new FormData(form);
            perform(async () => {
              await api('/accounts/password', 'POST', {
                username: d.get('username'),
                password: d.get('password'),
              });
              form.reset();
            });
          }}
        >
          <Select name="username" aria-label="修改密码的账号">
            <option value="viewer">viewer · 团队只读</option>
            <option value="admin">admin · 管理员</option>
          </Select>
          <input
            name="password"
            type="password"
            autoComplete="new-password"
            placeholder="新密码，10～72 字节"
            minLength={10}
            maxLength={72}
            required
          />
          <Button disabled={pending} variant="outline">
            更新密码
          </Button>
        </form>
      </section>
    </>
  );
}
