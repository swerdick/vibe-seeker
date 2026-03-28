import { useState, useCallback } from "react";
import { postSync } from "../utils/api";

interface UseSyncActionOptions {
  errorMessage: string;
  refetch?: () => void;
}

interface UseSyncActionResult<T> {
  execute: () => void;
  syncing: boolean;
  error: string | null;
  result: T | null;
}

export function useSyncAction<T = unknown>(
  url: string,
  { errorMessage, refetch }: UseSyncActionOptions,
): UseSyncActionResult<T> {
  const [syncing, setSyncing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<T | null>(null);

  const execute = useCallback(() => {
    setSyncing(true);
    setError(null);
    setResult(null);
    postSync(url)
      .then((data) => {
        setResult(data as T);
        refetch?.();
      })
      .catch(() => setError(errorMessage))
      .finally(() => setSyncing(false));
  }, [url, errorMessage, refetch]);

  return { execute, syncing, error, result };
}
