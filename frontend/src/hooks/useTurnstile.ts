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

    // If the script already loaded (e.g., React StrictMode re-mount), render the widget directly.
    const turnstile = (window as unknown as Record<string, unknown>).turnstile as
      | { render: (el: string, opts: Record<string, unknown>) => void }
      | undefined;
    if (turnstile) {
      turnstile.render("#turnstile-widget", renderOpts);
      return;
    }

    // Prevent loading the Turnstile script twice.
    if (document.querySelector('script[src*="challenges.cloudflare.com/turnstile"]')) {
      return;
    }

    const script = document.createElement("script");
    script.src =
      "https://challenges.cloudflare.com/turnstile/v0/api.js?onload=onTurnstileLoad";
    script.async = true;

    (window as unknown as Record<string, unknown>).onTurnstileLoad = () => {
      const ts = (window as unknown as Record<string, unknown>).turnstile as {
        render: (
          el: string,
          opts: {
            sitekey: string;
            callback: (token: string) => void;
            "error-callback": () => void;
          },
        ) => void;
      };
      ts.render("#turnstile-widget", renderOpts);
    };

    document.head.appendChild(script);
  }, [enabled]);

  return { authenticated, loading, error };
}
