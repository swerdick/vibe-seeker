import { useState, useEffect, useCallback } from "react";
import { fetchVibe } from "../utils/api";

interface UseVibesResult {
  genres: Record<string, number> | null;
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

export function useVibes(enabled: boolean): UseVibesResult {
  const [genres, setGenres] = useState<Record<string, number> | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);
  const [fetchKey, setFetchKey] = useState(0);

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    fetchVibe()
      .then((data) => {
        if (!cancelled) {
          setGenres(data.genres);
          setError(null);
          setPending(false);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setGenres(null);
          setError("Failed to load vibes.");
          setPending(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [enabled, fetchKey]);

  const refetch = useCallback(() => {
    setPending(true);
    setError(null);
    setFetchKey((k) => k + 1);
  }, []);

  const loading = pending || (enabled && genres === null && error === null);

  return { genres, loading, error, refetch };
}
