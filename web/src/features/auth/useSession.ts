import { useCallback, useEffect, useState } from 'react';
import { api, setCSRF } from '../../api/client';
import type { User } from '../../types';

export function useSession() {
  const [user, setUser] = useState<User | null>(null);
  const [ready, setReady] = useState(false);
  const signIn = useCallback((value: User) => {
    setCSRF(value.csrf);
    setUser(value);
  }, []);
  const clearSession = useCallback(() => {
    setCSRF('');
    setUser(null);
  }, []);

  useEffect(() => {
    let active = true;
    api<User>('/me')
      .then((value) => {
        if (active) signIn(value);
      })
      .catch(() => {})
      .finally(() => {
        if (active) setReady(true);
      });
    window.addEventListener('auth-expired', clearSession);
    return () => {
      active = false;
      window.removeEventListener('auth-expired', clearSession);
    };
  }, [signIn, clearSession]);

  const signOut = useCallback(async () => {
    await api('/logout', 'POST', {});
    clearSession();
  }, [clearSession]);

  return { user, ready, signIn, signOut };
}
