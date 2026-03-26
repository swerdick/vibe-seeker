# The Vibe Algorithm

## Core Concept: Layered Enrichment

The matching algorithm works because three different data sources each contribute a different piece of the puzzle:

1. **Spotify** tells us *who* you listen to (your top artists, ranked by listening frequency)
2. **Last.fm** tells us *what that music is like* (community-sourced tags: genres, moods, eras, descriptors)
3. **Ticketmaster** tells us *where artists play* (venues, show dates, booking history)

No single source has the complete picture. Spotify knows your taste but deprecated genre data. Last.fm has rich genre tags 
but doesn't know your listening habits. Ticketmaster has venue/event data but uses a very coarse genre taxonomy. The algorithm 
layers them together

## User Vibes

A user's vibe profile is built from their Spotify top artists:

1. Fetch top 50 artists for `medium_term` (~6 months) and `short_term` (~4 weeks)
2. Each artist gets a weight based on:
   - **Rank position**: Artist #1 gets weight 1.0, artist #50 gets 0.02 (linear decay at 0.02/position)
   - **Time range**: Medium-term artists get 1.0× multiplier, short-term gets 0.5× (established taste outweighs recent 
listening)
3. For each artist, fetch their Last.fm tags (filtered to relevance score ≥ 20, blocklisted noise tags removed)
4. Each tag accumulates weight across all artists: `tag_weight += artist_weight × (tag_relevance / 100)`
5. Normalize so the strongest tag = 1.0

The result is a sparse vector like: `{rock: 1.0, indie: 0.82, folk: 0.45, electronic: 0.31, ...}`

## Venue Vibes

A venue's vibe profile is built from its show history:

1. Get all artists who have played (or are booked to play) at the venue
2. Each artist gets a weight based on **recency** of their most recent show:
   - Last 3 months: 1.0
   - 3-6 months: 0.7
   - 6-12 months: 0.4
   - 12+ months: 0.2
3. Fetch Last.fm tags for each artist (same pipeline as user vibes, using the shared DB cache)
4. Accumulate and normalize the same way

This means a venue that books a lot of indie rock acts recently will have a strong indie rock vibe, even if they had a 
jazz residency two years ago.

## Tag Data Pipeline

Getting clean tag data is the hardest part. The pipeline has three layers of fallback:

```
1. Check DB cache (artist_tags table, 15-day TTL)
   ├─→ Hit? → use cached data
   └─→ Miss? → continue

2. Query Last.fm artist.getTopTags
   ├─→ Got tags? → cache and use
   ├─→ Zero results + diacritics in name? → retry with stripped diacritics (ü→u, é→e)
   │   └─→ Got tags now? → cache and use
   └─→ Still zero? → continue

3. Fall back to Ticketmaster classifications
   ├─→ Look up show_classifications for this artist's events
   ├─→ Map TM genre/sub-genre to tags (lowercased, with reduced weight)
   ├─→ Cache with source="ticketmaster"
   └─→ Still nothing? → artist contributes no signal (logged)
```

Last.fm tags are high-specificity (e.g., "dream pop", "atmospheric", "melancholic") and get full weight. Ticketmaster 
classifications are broad (e.g., "Rock", "Alternative Rock") and are intentionally weighted lower (~60-80% of a Last.fm 
top tag) to avoid overwhelming the nuanced Last.fm signal.

Tag keys are normalized to lowercase across all sources to prevent "Rock" and "rock" from becoming separate dimensions.

## Matching: Cosine Similarity

The match between a user and a venue is [cosine similarity](https://en.wikipedia.org/wiki/Cosine_similarity) between their vibe vectors:

```
similarity = (user · venue) / (||user|| × ||venue||)
```

This produces a score from 0.0 (no overlap) to 1.0 (identical taste profiles). It's scale-invariant — a venue with 100 
shows and a venue with 5 shows are compared fairly as long as their relative tag distributions are similar.

The matching happens **client-side** in the browser. Both vectors are small (<100 tags each), so the computation is 
instant and can update in real-time as users toggle genre selections.

## Interactive Filtering

Users can click individual vibes on/off in the sidebar. Deselecting "electronic" removes it from the user's vector and 
immediately recalculates all venue scores. This lets users explore questions like "if I only care about rock and folk, 
which venues match best?" without re-syncing anything.

A minimum match threshold slider filters the map to show only venues above a certain score, reducing visual noise.

## Known Limitations

- **No historical event data from Ticketmaster**: The API only returns upcoming events. Venue profiles will improve over 
time as more events are ingested, but new venues start with thin profiles.
- **~14% of artists have no tag data**: About 174 out of 1,209 artists returned zero tags from Last.fm and fell back to 
Ticketmaster's coarser taxonomy. Very obscure or very new artists have no data anywhere.
- **Ticketmaster venue coverage is biased toward larger venues**: Small DIY spaces, basement shows, and pop-up events 
don't appear in Ticketmaster's data. This is the biggest coverage gap.
- **No boost factors yet**: The current matching is pure cosine similarity. Planned but not yet implemented: boosts for 
"show tonight", "price under $20", or "artist in your Spotify top 50 is playing".
