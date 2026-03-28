import type { User, VenueData } from "../types";

interface TagPrevalence {
  tag: string;
  prevalence: number;
}

interface TagRelation {
  tag: string;
  strength: number;
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
  });
}

export async function fetchVibe(): Promise<{
  genres: Record<string, number>;
  genre_count: number;
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
  const res = await fetch("/api/auth/anonymous", {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ turnstile_token: turnstileToken }),
  });
  if (!res.ok) throw new Error("anonymous login failed");
  return res.json();
}
