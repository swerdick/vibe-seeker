import { useEffect, useState, useMemo, useCallback } from "react";
import TopBar from "../components/TopBar";
import VenueMap from "../components/VenueMap";
import VibeGraph from "../components/VibeGraph";
import type { VenueData } from "../types";
import { useVibeGraph } from "../hooks/useVibeGraph";
import { useVenueMatching } from "../hooks/useVenueMatching";
import { anonymousLogin } from "../utils/api";

export default function Explore() {
  const [authenticated, setAuthenticated] = useState(false);
  const [captchaLoading, setCaptchaLoading] = useState(true);
  const [captchaError, setCaptchaError] = useState<string | null>(null);
  const [venues, setVenues] = useState<VenueData[]>([]);
  const [selectedVenue, setSelectedVenue] = useState<VenueData | null>(null);
  const [minMatch, setMinMatch] = useState(0);

  const graph = useVibeGraph(null, authenticated);

  // Build a vibes record from graph nodes for matching.
  const vibesFromGraph = useMemo(() => {
    const vibes: Record<string, number> = {};
    for (const node of graph.nodes) {
      vibes[node.id] = node.prevalence;
    }
    return Object.keys(vibes).length > 0 ? vibes : null;
  }, [graph.nodes]);

  const { venueScores, visibleVenues } = useVenueMatching(
    vibesFromGraph,
    graph.selectedTags,
    venues,
    minMatch,
  );

  const loadTurnstile = useCallback(() => {
    const siteKey = import.meta.env.VITE_TURNSTILE_SITE_KEY;
    if (!siteKey) {
      setCaptchaError("Turnstile site key not configured");
      setCaptchaLoading(false);
      return;
    }

    // If the script already loaded (e.g., React StrictMode re-mount), render the widget directly.
    const turnstile = (window as unknown as Record<string, unknown>).turnstile as
      | { render: (el: string, opts: Record<string, unknown>) => void }
      | undefined;
    if (turnstile) {
      turnstile.render("#turnstile-widget", {
        sitekey: siteKey,
        callback: (token: string) => {
          anonymousLogin(token)
            .then(() => {
              setAuthenticated(true);
              setCaptchaLoading(false);
            })
            .catch(() => {
              setCaptchaError("Verification failed. Please try again.");
              setCaptchaLoading(false);
            });
        },
        "error-callback": () => {
          setCaptchaError("Captcha failed to load.");
          setCaptchaLoading(false);
        },
      });
      return;
    }

    // Prevent loading the Turnstile script twice.
    if (document.querySelector('script[src*="challenges.cloudflare.com/turnstile"]')) {
      return;
    }

    const script = document.createElement("script");
    script.src = "https://challenges.cloudflare.com/turnstile/v0/api.js?onload=onTurnstileLoad";
    script.async = true;

    (window as unknown as Record<string, unknown>).onTurnstileLoad = () => {
      const turnstile = (window as unknown as Record<string, unknown>).turnstile as {
        render: (
          el: string,
          opts: { sitekey: string; callback: (token: string) => void; "error-callback": () => void },
        ) => void;
      };

      turnstile.render("#turnstile-widget", {
        sitekey: siteKey,
        callback: (token: string) => {
          anonymousLogin(token)
            .then(() => {
              setAuthenticated(true);
              setCaptchaLoading(false);
            })
            .catch(() => {
              setCaptchaError("Verification failed. Please try again.");
              setCaptchaLoading(false);
            });
        },
        "error-callback": () => {
          setCaptchaError("Captcha failed to load.");
          setCaptchaLoading(false);
        },
      });
    };

    document.head.appendChild(script);
    setCaptchaLoading(true);
  }, []);

  // Check if we already have a session, otherwise load Turnstile.
  useEffect(() => {
    fetch("/api/auth/me", { credentials: "include" })
      .then((res) => {
        if (res.ok) {
          setAuthenticated(true);
          setCaptchaLoading(false);
        } else {
          loadTurnstile();
        }
      })
      .catch(() => loadTurnstile());
  }, [loadTurnstile]);

  // Fetch venues once authenticated.
  useEffect(() => {
    if (!authenticated) return;
    fetch("/api/venues", { credentials: "include" })
      .then((res) => {
        if (!res.ok) throw new Error("failed to load venues");
        return res.json();
      })
      .then((data: { venues: VenueData[]; count: number }) => {
        setVenues(data.venues || []);
      })
      .catch(() => {
        setVenues([]);
      });
  }, [authenticated]);

  // Show captcha screen before authenticated.
  if (!authenticated) {
    return (
      <div className="page">
        <h1>Vibe Seeker</h1>
        <p>Discover venues that match your vibe.</p>
        {captchaLoading && <p>Verifying...</p>}
        {captchaError && <p className="error">{captchaError}</p>}
        <div id="turnstile-widget" />
      </div>
    );
  }

  return (
    <div className="app-layout">
      <TopBar anonymous />
      <div className="main-content">
        <VenueMap
          venues={visibleVenues}
          venueScores={venueScores}
          selectedVenue={selectedVenue}
          minMatch={minMatch}
          visibleCount={visibleVenues.length}
          onSelectVenue={setSelectedVenue}
          onClosePopup={() => setSelectedVenue(null)}
          onMinMatchChange={setMinMatch}
        />
        <VibeGraph
          nodes={graph.nodes}
          edges={graph.edges}
          newNodeIds={graph.newNodeIds}
          selectedCount={graph.selectedTags.size}
          totalCount={graph.nodes.length}
          onToggleNode={graph.toggleNode}
          onAddNode={graph.addNode}
          onReset={graph.reset}
          onSelectAll={graph.selectAll}
          onSelectNone={graph.selectNone}
        />
      </div>
    </div>
  );
}
