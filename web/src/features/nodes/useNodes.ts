import { useCallback, useEffect, useRef, useState } from 'react';
import { api } from '../../api/client';
import type { Node, User } from '../../types';

export function useNodes(user: User | null) {
  const [nodes, setNodes] = useState<Node[]>([]);
  const [updated, setUpdated] = useState(0);
  const [error, setError] = useState('');
  const generation = useRef(0);
  const latestRequest = useRef(0);

  const refresh = useCallback(async () => {
    if (!user) return;
    const session = generation.current;
    const request = ++latestRequest.current;
    const current = () => session === generation.current && request === latestRequest.current;
    try {
      const data = await api<Node[]>('/nodes');
      if (!current()) return;
      setNodes(data);
      setUpdated(Date.now());
      setError('');
    } catch (error) {
      if (current()) setError((error as Error).message);
    }
  }, [user]);

  useEffect(() => {
    generation.current++;
    if (!user) {
      setNodes([]);
      setUpdated(0);
      setError('');
      return;
    }
    let disposed = false;
    let timer: ReturnType<typeof setTimeout>;
    const tick = async () => {
      await refresh();
      if (!disposed) timer = setTimeout(tick, 5000);
    };
    void tick();
    return () => {
      disposed = true;
      generation.current++;
      clearTimeout(timer);
    };
  }, [user, refresh]);

  return { nodes, updated, error, setError, refresh };
}
