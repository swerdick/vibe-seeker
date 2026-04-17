import type { User, VenueData } from "../types";

interface TagPrevalence {
  tag: string;
  prevalence: number;
}

interface TagRelation {
  tag: string;
  strength: number;
}

// SHA256 of empty string — used for POST requests with no body.
const EMPTY_HASH =
  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855";

// CloudFront OAC for Lambda URLs requires x-amz-content-sha256 on POST/PUT.
// The header contains the SHA256 hash of the request body so CloudFront can
// include it in its SigV4 signature.
async function contentHash(body?: string): Promise<string> {
  if (!body) return EMPTY_HASH;
  const encoded = new TextEncoder().encode(body);
  const digest = await crypto.subtle.digest("SHA-256", encoded);
  return Array.from(new Uint8Array(digest))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

export async function fetchAuthMe(): Promise<User> {
  const res = await fetch("/api/auth/me", { credentials: "include" });
  if (!res.ok) throw new Error("unauthorized");
  return res.json();
}

export async function postLogout(): Promise<void> {
  await fetch("/api/auth/logout", {
    method: "POST",
    credentials: "include",
    headers: { "x-amz-content-sha256": EMPTY_HASH },
  });
}

export async function fetchVibe(): Promise<{
  vibes: Record<string, number>;
  vibe_count: number;
}> {
  const res = await fetch("/api/vibe", { credentials: "include" });
  if (!res.ok) throw new Error("failed to load vibe");
  return res.json();
}

export async function fetchVenues(): Promise<{
  venues: VenueData[];
  count: number;
}> {
  const res = await fetch("/api/venues", { credentials: "include" });
  if (!res.ok) throw new Error("failed to load venues");
  return res.json();
}

export async function postSync(url: string): Promise<unknown> {
  const res = await fetch(url, {
    method: "POST",
    credentials: "include",
    headers: { "x-amz-content-sha256": EMPTY_HASH },
  });
  if (!res.ok) throw new Error("sync failed");
  return res.json();
}

export async function fetchTopVibes(
  limit = 10,
): Promise<TagPrevalence[]> {
  const res = await fetch(`/api/vibes/top?limit=${limit}`, {
    credentials: "include",
  });
  if (!res.ok) throw new Error("failed to fetch top vibes");
  const data: { vibes: TagPrevalence[] } = await res.json();
  return data.vibes ?? [];
}

export async function fetchRelatedVibes(
  tag: string,
  limit = 8,
): Promise<TagRelation[]> {
  const res = await fetch(
    `/api/vibes/related?tag=${encodeURIComponent(tag)}&limit=${limit}`,
    { credentials: "include" },
  );
  if (!res.ok) throw new Error("failed to fetch related vibes");
  const data: { tag: string; related: TagRelation[] } = await res.json();
  return data.related ?? [];
}

export async function anonymousLogin(
  turnstileToken: string,
): Promise<{ spotify_id: string; display_name: string }> {
  const body = JSON.stringify({ turnstile_token: turnstileToken });
  const res = await fetch("/api/auth/anonymous", {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      "x-amz-content-sha256": await contentHash(body),
    },
    body,
  });
  if (!res.ok) throw new Error("anonymous login failed");
  return res.json();
}
