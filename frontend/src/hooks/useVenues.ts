import { useState, useEffect, useCallback } from "react";
import { fetchVenues as apiFetchVenues } from "../utils/api";
import type { VenueData } from "../types";

interface UseVenuesResult {
  venues: VenueData[];
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

export function useVenues(enabled: boolean): UseVenuesResult {
  const [venues, setVenues] = useState<VenueData[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);
  const [fetchKey, setFetchKey] = useState(0);
  const [fetched, setFetched] = useState(false);

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    apiFetchVenues()
      .then((data) => {
        if (!cancelled) {
          setVenues(data.venues || []);
          setError(null);
          setPending(false);
          setFetched(true);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setVenues([]);
          setError("Failed to load venues.");
          setPending(false);
          setFetched(true);
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

  const loading = pending || (enabled && !fetched);

  return { venues, loading, error, refetch: refetch };
}
