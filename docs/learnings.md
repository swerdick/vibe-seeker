# Learnings

Things I learned building Vibe Seeker that I wish I'd known before starting.

## Data Quality is an Issue

Documentation describes an idealized version of the API. The actual data tells a different story.

**Spotify deprecated genre data mid-build.** The `genres` field on artist objects — documented, still present in responses 
— returns empty arrays. No announcement in the changelog at the time. I found out when my taste vectors came back empty. 
The fix was pivoting to Last.fm's community-sourced tags, which turned out to be *better* than Spotify's genres anyway 
(richer taxonomy, mood/vibe tags, not just genre labels).

**Ticketmaster's venue data is full of ghosts.** A geo search for NYC venues returns ~945 results. Only 55 have active events. 
The rest are defunct venues (closed years ago), non-music spaces (conference centers, cruise ships, schools), and duplicate 
entries. Webster Hall — one of NYC's most active music venues — appeared in our DB under an inactive venue ID with zero 
events, while the active ID wasn't returned by the geo search at all. Some international venues (Spain, Mexico) appeared
in NYC results due to bad geocoding in Ticketmaster's database.

**Rate limits don't always match documentation.** Ticketmaster documents 5 req/sec, but we hit 429s well before that 
threshold when fetching events for 900+ venues sequentially. The solution was adaptive rate limiting: start at 5 req/sec, 
back off by 1 second on every 429, speed back up on success. Simple but necessary.

## The Data Source Landscape Is Hostile to Hobbyists

Before building, I assumed I'd have 3-4 event data sources to pull from. The reality:

| Source           | Status  | Why                                                               |
|------------------|---------|-------------------------------------------------------------------|
| Ticketmaster     | Works   | Free tier, decent API, data requires de-dupes and double checking |
| Last.fm          | Works   | Free, great tag data, rate limited                                |
| SeatGeek         | Pending | Developer account approval required (still pending)               |
| Bandsintown      | Blocked | API restricted to artist managers only                            |
| Songkick         | Blocked | $500 per million requests                                         |
| Resident Advisor | Blocked | Returns 403 to any automated request                              |

The small venue data that would make this app truly special (DIY spaces, basement shows, $10 cover charges) is exactly
the data that's hardest to get programmatically. The venues that need discovery the most are the ones least likely to be 
in any API

## Cache Everything

The read-through cache for Last.fm tags transformed the app from "unusably slow" to "fast on repeat visits."

First vibe sync: ~4 minutes (1,200 Last.fm API calls at 200ms each). Second sync: ~5 seconds (85% cache hits, only ~170 
API calls for new artists). The cache is a PostgreSQL table with a 15-day TTL — simple, durable, and doubles as integration
test fixtures

The same cache is shared between user vibe computation and venue vibe computation, so an artist who appears in both your
Spotify top 50 and a venue's show history only gets looked up once.


## Multi-Source Data Requires Normalization at Every Layer

When you pull data from Spotify, Last.fm, and Ticketmaster, nothing agrees:

- **Artist names**: Ticketmaster sends "RÜFÜS DU SOL" (with umlauts). Last.fm only recognizes "RUFUS DU SOL." Solution:
  diacritics stripping as a retry strategy
- **Genre taxonomy**: Last.fm uses fine-grained community tags ("dream pop", "shoegaze"). Ticketmaster uses a coarse hierarchy
  ("Music > Rock > Alternative Rock"). Solution: normalize everything to lowercase, weight Ticketmaster's broad genres lower
  than Last.fm's specific tags
- **Venue identity**: Ticketmaster has duplicate venue entries for the same physical location with different IDs. Solution:
  source-prefixed IDs (`tm_`) and accepting that some duplicates slip through.
  - As data sources expand I'll need to try to de-dupe by things like addresses, and normalized venue names, and lat/long
    if available
- **Data freshness**: Spotify tokens expire hourly. Last.fm tags are stable for weeks (or probably more). Ticketmaster events
  change daily. Solution: different TTLs for different data types (tokens: refresh on use, tags: 15 days, venues: 6 hours)

## The Algorithm Is Simple — The Data Pipeline Is Hard

The actual matching math is ~30 lines of code (cosine similarity on two maps). The data pipeline to get clean, normalized, 
cached tag vectors from three different APIs with different auth models, rate limits, and data quality issues is ~2,000 
lines. Getting the data right is 80% of the work
