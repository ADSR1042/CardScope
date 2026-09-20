let csrf = '';
export async function api<T>(path: string, method = 'GET', body?: unknown): Promise<T> {
  const r = await fetch('/api/v1' + path, {
    method,
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json', ...(csrf ? { 'X-CSRF-Token': csrf } : {}) },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const d = await r.json();
  if (!r.ok) {
    if (r.status === 401 && path != '/login') window.dispatchEvent(new Event('auth-expired'));
    throw new Error(d.error || '请求失败');
  }
  return d;
}

export function setCSRF(value: string) {
  csrf = value;
}
