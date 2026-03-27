interface TagPrevalence {
  tag: string;
  prevalence: number;
}

interface TagRelation {
  tag: string;
  strength: number;
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
