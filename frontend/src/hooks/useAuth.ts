import { useState, useEffect, useCallback } from "react";
import type { User } from "../types";
import { fetchAuthMe, postLogout } from "../utils/api";

export interface AuthState {
  user: User | null;
  loading: boolean;
  authenticated: boolean;
  logout: () => Promise<void>;
  refresh: () => void;
}

export function useAuth(): AuthState {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshKey, setRefreshKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    fetchAuthMe()
      .then((data) => {
        if (!cancelled) setUser(data);
      })
      .catch(() => {
        if (!cancelled) setUser(null);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [refreshKey]);

  const refresh = useCallback(() => {
    setRefreshKey((k) => k + 1);
  }, []);

  const logout = useCallback(async () => {
    await postLogout();
    setUser(null);
  }, []);

  return {
    user,
    loading,
    authenticated: user !== null,
    logout,
    refresh,
  };
}
