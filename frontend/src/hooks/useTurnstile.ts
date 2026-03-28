import { useState, useEffect, useRef } from "react";
import { anonymousLogin } from "../utils/api";

interface UseTurnstileOptions {
  enabled: boolean;
  onAuthenticated?: () => void;
}

interface UseTurnstileResult {
  authenticated: boolean;
  loading: boolean;
  error: string | null;
}

export function useTurnstile({
  enabled,
  onAuthenticated,
}: UseTurnstileOptions): UseTurnstileResult {
  const [authenticated, setAuthenticated] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const onAuthRef = useRef(onAuthenticated);

  useEffect(() => {
    onAuthRef.current = onAuthenticated;
  }, [onAuthenticated]);

  useEffect(() => {
    if (!enabled) return;

    const siteKey = import.meta.env.VITE_TURNSTILE_SITE_KEY;
    if (!siteKey) {
      // Defer to avoid synchronous setState in effect body.
      Promise.resolve().then(() => {
        setError("Turnstile site key not configured");
        setLoading(false);
      });
      return;
    }

    const onSuccess = (token: string) => {
      anonymousLogin(token)
        .then(() => {
          setAuthenticated(true);
          setLoading(false);
          onAuthRef.current?.();
        })
        .catch(() => {
          setError("Verification failed. Please try again.");
          setLoading(false);
        });
    };

    const onError = () => {
      setError("Captcha failed to load.");
      setLoading(false);
    };

    const renderOpts = {
      sitekey: siteKey,
      callback: onSuccess,
      "error-callback": onError,
    };

    const win = window as unknown as Record<string, unknown>;

    const handleLoad = () => {
      const ts = win.turnstile as
        | { render: (el: string, opts: Record<string, unknown>) => void }
        | undefined;
      if (ts) ts.render("#turnstile-widget", renderOpts);
    };

    // If the script already loaded (e.g., React StrictMode re-mount), render the widget directly.
    if (win.turnstile) {
      handleLoad();
      return;
    }

    // Always register the onload callback so the widget renders when the script finishes,
    // even if the script tag was already added by a previous mount.
    win.onTurnstileLoad = handleLoad;

    // Only append the script tag if it doesn't already exist.
    if (!document.querySelector('script[src*="challenges.cloudflare.com/turnstile"]')) {
      const script = document.createElement("script");
      script.src =
        "https://challenges.cloudflare.com/turnstile/v0/api.js?onload=onTurnstileLoad";
      script.async = true;
      document.head.appendChild(script);
    }

    return () => {
      if (win.onTurnstileLoad === handleLoad) {
        delete win.onTurnstileLoad;
      }
    };
  }, [enabled]);

  return { authenticated, loading, error };
}
