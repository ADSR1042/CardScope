import { useState } from 'react';
import { Server, X } from 'lucide-react';
import { api } from '../../api/client';
import { Badge } from '../../components/Badge';
import { Button } from '../../components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '../../components/ui/dialog';
import type { Node } from '../../types';

export function NodeSettings({
  node: n,
  pending,
  perform,
  onCode,
}: {
  node: Node;
  pending: boolean;
  perform: (
    fn: () => Promise<unknown>,
    refreshAfter?: boolean,
    onError?: (message: string) => void,
  ) => Promise<void>;
  onCode: (code: string) => void;
}) {
  const [nets, setNets] = useState(n.networks || []),
    [confirm, setConfirm] = useState('');
  const [tags, setTags] = useState(n.tags || []);
  const [tagInput, setTagInput] = useState('');
  const [tagError, setTagError] = useState('');
  const [saveError, setSaveError] = useState('');
  const nameError = saveError.includes('节点名称');
  const collectTags = () => {
    const value = tagInput.trim();
    if (value && ([...value].length > 32 || /[\u0000-\u001f\u007f-\u009f]/.test(value))) {
      setTagError('标签最多 32 个字符，不能包含控制字符');
      return null;
    }
    const next = value ? [...new Set([...tags, value])] : tags;
    if (next.length > 20) {
      setTagError('每个节点最多 20 个标签');
      return null;
    }
    setTagError('');
    setTags(next);
    setTagInput('');
    return next;
  };
  return (
    <section className="settings-panel">
      <div className="section-line">
        <h3>
          <Server size={18} />
          {n.name}
        </h3>
        <Badge tone={n.enrolled ? 'green' : 'neutral'}>
          {n.enrolled ? '凭据有效' : '待接入 / 已撤销'}
        </Badge>
      </div>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          const d = new FormData(e.currentTarget);
          const nextTags = collectTags();
          if (!nextTags) return;
          setSaveError('');
          perform(
            () =>
              api('/nodes/' + n.id, 'PATCH', {
                name: d.get('name'),
                expected: Number(d.get('expected')),
                networks: nets,
                tags: nextTags,
              }),
            true,
            setSaveError,
          );
        }}
      >
        <div className="settings-fields">
          <label className="form-field">
            节点名称
            <input
              name="name"
              aria-label="节点名称"
              defaultValue={n.name}
              maxLength={128}
              required
              aria-invalid={nameError}
              aria-describedby={nameError ? `name-error-${n.id}` : undefined}
              onChange={() => setSaveError('')}
            />
            {nameError && (
              <span id={`name-error-${n.id}`} role="alert" className="field-error">
                {saveError}
              </span>
            )}
          </label>
          <label className="form-field">
            预期 GPU 数（0 表示不检查）
            <input name="expected" type="number" min="0" max="128" defaultValue={n.expected} />
          </label>
        </div>
        <div className="tag-editor">
          <label className="field-label" htmlFor={`tags-${n.id}`}>
            节点标签
          </label>
          <div className="node-tags">
            {tags.map((tag) => (
              <span className="node-tag" key={tag}>
                {tag}
                <button
                  type="button"
                  aria-label={`删除标签 ${tag}`}
                  disabled={pending}
                  onClick={() => setTags(tags.filter((value) => value !== tag))}
                >
                  <X size={12} />
                </button>
              </span>
            ))}
          </div>
          <div className="tag-input-row">
            <input
              id={`tags-${n.id}`}
              value={tagInput}
              disabled={pending}
              placeholder="例如：训练、研发组、机房 A"
              aria-describedby={`tags-help-${n.id}`}
              onChange={(e) => {
                setTagInput(e.target.value);
                setTagError('');
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.nativeEvent.isComposing) {
                  e.preventDefault();
                  collectTags();
                }
              }}
            />
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={pending || !tagInput.trim()}
              onClick={collectTags}
            >
              添加标签
            </Button>
          </div>
          <small id={`tags-help-${n.id}`} className="muted">
            输入后按回车添加，每个节点最多 20 个；点击保存配置后生效。
          </small>
          {tagError && (
            <p role="alert" className="error-text">
              {tagError}
            </p>
          )}
        </div>
        <label className="field-label">汇总网卡（不选则只展示主要接口）</label>
        <div className="network-choices">
          {(n.snapshot.system.networks || []).map((v) => (
            <label key={v.name}>
              <input
                type="checkbox"
                checked={nets.includes(v.name)}
                onChange={(e) =>
                  setNets(e.target.checked ? [...nets, v.name] : nets.filter((x) => x !== v.name))
                }
              />
              {v.name}
            </label>
          ))}
          {!n.snapshot.system.networks?.length && (
            <span className="muted">等待节点上报网卡信息</span>
          )}
        </div>
        <div className="settings-actions">
          {saveError && !nameError && (
            <p role="alert" className="field-error">
              {saveError}
            </p>
          )}
          <Button disabled={pending} type="submit" variant="outline" size="sm">
            保存配置
          </Button>
          <Button
            disabled={pending}
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => setConfirm('enrollment')}
          >
            重新签发接入码
          </Button>
          <Button
            disabled={pending}
            type="button"
            variant="destructive"
            size="sm"
            onClick={() => setConfirm('revoke')}
          >
            撤销凭据
          </Button>
        </div>
      </form>
      <Dialog
        open={!!confirm}
        onOpenChange={(v) => {
          if (!v) setConfirm('');
        }}
      >
        <DialogContent>
          <DialogTitle className="dialog-title">
            {confirm === 'revoke' ? '撤销节点凭据？' : '重新签发接入码？'}
          </DialogTitle>
          <DialogDescription className="muted">
            {n.name} 的现有凭据会立即失效。历史数据保留，重新接入后继续采集。
          </DialogDescription>
          <div className="dialog-actions">
            <Button variant="outline" onClick={() => setConfirm('')}>
              取消
            </Button>
            <Button
              onClick={() =>
                perform(async () => {
                  const d = await api<{ code?: string }>(`/nodes/${n.id}/${confirm}`, 'POST', {});
                  setConfirm('');
                  if (d.code) onCode(d.code);
                })
              }
              disabled={pending}
            >
              确认
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </section>
  );
}
