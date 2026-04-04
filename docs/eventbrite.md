# EventBrite API Research

Research conducted 2026-04-02. Based on live API calls using a private OAuth token.

## TL;DR

EventBrite's data is **much more structured than expected**. Artist names are typically in event titles (comma-separated), not buried in free-text descriptions. Rich typed tags provide category, subcategory, format, and organizer-defined genre tags. The main limitation is discovery — the public search API was deprecated in 2019, but a `POST /v3/destination/search/` endpoint still works and returns 10,000+ NYC events.

---

## Authentication

- **Method:** OAuth 2.0 Bearer token
- **Header:** `Authorization: Bearer <token>`
- Token obtained from EventBrite Developer Portal
- Same token works for all endpoints tested

## Rate Limits

- **1,000 calls per hour** per OAuth token
- **48,000 calls per day** maximum
- Returns HTTP 429 on excess

---

## Event Discovery

### Deprecated: `/v3/events/search/`

Removed December 2019. Returns 404. **Cannot search public events this way.**

### Working: `POST /v3/destination/search/`

Undocumented but functional. Returns paginated event results for a geographic area.

**Request:**
```json
POST /v3/destination/search/
{
  "event_search": {
    "dates": "current_future",
    "dedup": true,
    "places": ["85977539"],           // Who's On First ID for NYC
    "page_size": 50
  },
  "expand.destination_event": [
    "primary_venue",
    "ticket_availability",
    "event_sales_status"
  ]
}
```

**Pagination:** Uses continuation tokens, not page numbers.
```json
"pagination": {
  "object_count": 10000,
  "page_size": 50,
  "continuation": "eyJwYWdlIjoyfQ"
}
```

**Notes:**
- Returns ALL event categories (not just music) — filter client-side using tags
- `object_count` caps at 10,000 (may be more events)
- Category filtering via `"categories": ["103"]` in the request body returned errors — needs client-side filtering

### Working: `GET /v3/venues/{id}/events/`

Returns events at a specific venue. Requires knowing the venue ID.

```
GET /v3/venues/279431833/events/?status=live&order_by=start_asc
```

**Limitation:** Elsewhere (one venue, three rooms) uses different venue IDs per room, so a single venue ID only returns events for that room.

### Blocked: `GET /v3/organizations/{id}/events/`

Returns 403 — only accessible by the organization's own OAuth token.

---

## Event Object Structure

### Via `/v3/events/{id}/` (full detail endpoint)

```json
{
  "name": { "text": "Frog, Olivia O., DJ Silky Smooth" },
  "description": { "text": "A concert w/ Frog in The Hall @ Elsewhere..." },
  "summary": "A concert w/ Frog in The Hall @ Elsewhere in Bushwick...",
  "url": "https://www.eventbrite.com/e/...",
  "start": { "local": "2026-04-17T19:00:00", "utc": "...", "timezone": "America/New_York" },
  "end": { ... },
  "category_id": "103",
  "subcategory_id": "3009",
  "format_id": "6",
  "venue_id": "296431794",
  "organizer_id": "105655500371",
  "organization_id": "2569518481181",
  "status": "live",
  "is_free": false,
  "online_event": false,
  "capacity": null,
  "logo": { "url": "...", "original": { ... } }
}
```

**With `?expand=venue,category,subcategory,format,organizer`:**

```json
{
  "category": { "id": "103", "name": "Music" },
  "subcategory": { "id": "3009", "name": "Indie" },
  "format": { "id": "6", "name": "Concert or Performance" },
  "venue": {
    "name": "Elsewhere - The Hall",
    "latitude": "40.709411",
    "longitude": "-73.923169",
    "address": {
      "address_1": "599 Johnson Avenue",
      "city": "Brooklyn",
      "region": "NY",
      "postal_code": "11237",
      "country": "US"
    }
  },
  "organizer": {
    "name": "Elsewhere",
    "website": "https://www.elsewhere.club/"
  }
}
```

**There is NO structured performer/artist field.** Artist names exist only in event titles and descriptions.

### Via `POST /v3/destination/search/` (discovery endpoint)

Returns a different, flatter object. Key difference: **typed `tags` array**.

```json
{
  "id": "1983103228139",
  "name": "Frog, Olivia O., DJ Silky Smooth",
  "summary": "A concert w/ Frog in The Hall @ Elsewhere...",
  "url": "...",
  "start_date": "2026-04-17",
  "start_time": "19:00",
  "tags": [
    { "prefix": "EventbriteSubCategory", "tag": "EventbriteSubCategory/3009", "display_name": "Indie" },
    { "prefix": "EventbriteCategory", "tag": "EventbriteCategory/103", "display_name": "Music" },
    { "prefix": "EventbriteFormat", "tag": "EventbriteFormat/6", "display_name": "Concert or Performance" },
    { "prefix": "OrganizerTag", "tag": "OrganizerTag/Concert", "display_name": "Concert" },
    { "prefix": "OrganizerTag", "tag": "OrganizerTag/Rock", "display_name": "Rock" },
    { "prefix": "OrganizerTag", "tag": "OrganizerTag/Alternative", "display_name": "Alternative" },
    { "prefix": "OrganizerTag", "tag": "OrganizerTag/Indierock", "display_name": "Indierock" },
    { "prefix": "OrganizerTag", "tag": "OrganizerTag/Elsewhere", "display_name": "Elsewhere" }
  ],
  "primary_venue": { "name": "Elsewhere - The Hall", "id": "296431794" }
}
```

**Tag prefixes:**
| Prefix | Meaning | Example |
|--------|---------|---------|
| `EventbriteCategory` | Top-level category | Music, Business & Professional |
| `EventbriteSubCategory` | Genre/subcategory | Indie, Pop, Hip Hop / Rap |
| `EventbriteFormat` | Event format | Concert or Performance, Party |
| `OrganizerTag` | User-defined tags | Indierock, Electropop, Elsewhere |

---

## Categories & Subcategories

21 top-level categories. Music-relevant ones:

| ID | Category |
|----|----------|
| 103 | Music |
| 105 | Performing & Visual Arts |
| 104 | Film, Media & Entertainment |

### Music Subcategories (Category 103)

| ID | Name | | ID | Name |
|----|------|-|----|------|
| 3001 | Alternative | | 3017 | Rock |
| 3002 | Blues & Jazz | | 3018 | Top 40 |
| 3003 | Classical | | 3019 | Acoustic |
| 3004 | Country | | 3020 | Americana |
| 3005 | Cultural | | 3021 | Bluegrass |
| 3006 | EDM / Electronic | | 3022 | Blues |
| 3007 | Folk | | 3023 | DJ/Dance |
| 3008 | Hip Hop / Rap | | 3024 | EDM |
| 3009 | Indie | | 3025 | Electronic |
| 3010 | Latin | | 3026 | Experimental |
| 3011 | Metal | | 3027 | Jazz |
| 3012 | Opera | | 3028 | Psychedelic |
| 3013 | Pop | | 3029 | Punk/Hardcore |
| 3014 | R&B | | 3030 | Singer/Songwriter |
| 3015 | Reggae | | 3031 | World |
| 3016 | Religious/Spiritual | | 3999 | Other |

### Formats

| ID | Name | Relevant? |
|----|------|-----------|
| 6 | Concert or Performance | High — named artists likely |
| 11 | Party or Social Gathering | Medium — DJ/club nights, often no named artists |
| 5 | Festival or Fair | Medium — multi-artist |
| 16 | Tour | Medium |
| 100 | Other | Low |

---

## Data Patterns Observed

### Concert Events (Format 6: "Concert or Performance")

**Title pattern:** Comma-separated artist names, sometimes with `+` separator.

| Title | Artists |
|-------|---------|
| `Six Sex, deBasement` | Six Sex, deBasement |
| `Frog, Olivia O., DJ Silky Smooth` | Frog, Olivia O., DJ Silky Smooth |
| `WU LYF, Lauren Auder` | WU LYF, Lauren Auder |
| `ChaseWest, Dan Molinari, Twin Diplomacy, J. Richards, Soirée` | 5 artists |
| `Elsewhere Presents: Pearly Drops, Teather @ Market Hotel` | Pearly Drops, Teather |

**Description pattern (Elsewhere template):**
```
A concert w/ [HEADLINER] in [ROOM] @ Elsewhere in Bushwick on [DATE]!
Tickets start at $[PRICE] • [TIME] • [AGE]+ | Genre: [GENRE1], [GENRE2]
```

**OrganizerTags for concerts often include:** genre names (Indierock, Electropop, Poppunk, alternative_rock), venue names, location tags (Brooklyn, Nyc, Bushwick).

### Party/Club Events (Format 11: "Party or Social Gathering")

| Title | Pattern |
|-------|---------|
| `Friday Night Lights Everyone FREE B4 12am w/RSVP at Mama Taco Bk` | No artists, venue in title |
| `Matinee Disco @ Joyface` | Concept + venue |
| `LOVE 2 LOVE - A DONNA SUMMER DISCO PARTY` | Tribute/themed — referenced artist not performing |

OrganizerTags for parties: mood/vibe tags (Disco, 80s, Afrobeats, Danceparty, Nightlife).

### Edge Cases & Miscategorized Events

| Title | Listed Category | Actual Type |
|-------|----------------|-------------|
| `MALIE DONN LIVE IN NYC` | Seasonal & Holiday | Music performance |
| `SOLD OUT - Jesse Malin + John Varvatos: Almost Grown` | Hobbies (Books) | Book event (not music) |
| `Steve Schirripa + Michael Imperioli: WillieBoy Eats the World` | Hobbies (Books) | Book event |
| `Music Aperitivo: Joe Alterman & Mocean Worker` | Music | Live jazz at bookstore |

The `+` separator in titles creates ambiguity: it could mean two artists OR an artist + collaborator/author.

### Non-Music Events with Vibe Signal

Many non-music events still carry cultural/vibe tags that could inform venue profiles:
- Galas at City Winery → upscale, cultural
- Haitian parties → Caribbean, Konpa
- Art openings → Fine Art, contemporary
- Comedy shows → nightlife, entertainment

---

## Comparison with Ticketmaster

| Dimension | Ticketmaster | EventBrite |
|-----------|-------------|------------|
| **Artist data** | Structured `attractions` array | In event title (free text) |
| **Genre data** | `classifications` (segment/genre/subgenre) | Category + subcategory + OrganizerTags |
| **Genre granularity** | Coarse (e.g., "Rock", "Alternative") | Fine-grained organizer tags (e.g., "Indierock", "Electropop") |
| **Venue data** | Structured with lat/lon | Structured with lat/lon |
| **Discovery** | Geo search with pagination | Destination search (POST) |
| **Event types** | Primarily ticketed shows | Mix of free/paid, parties, concerts, non-music |
| **DIY/small venues** | Poor coverage | Better (parties, pop-ups, small spaces) |
| **Rate limits** | 5 req/sec (documented) | 1,000/hr, 48,000/day |
| **Data quality** | Duplicates, defunct venues | Miscategorized events, all-caps spam |

**Key advantage of EventBrite:** Covers the small/DIY/party scene that Ticketmaster misses — exactly the gap Vibe Seeker needs to fill.

**Key disadvantage:** No structured performer field means artist extraction requires parsing titles.

---

## Integration Approach (Recommended)

1. **Discovery:** `POST /v3/destination/search/` with NYC place ID, paginate via continuation tokens
2. **Filter:** Client-side — check for `EventbriteCategory/103` (Music) in tags
3. **Extract artists:** Parse event title (comma/+ separated) for Format 6 events
4. **Extract tags:** Combine `EventbriteSubCategory` + `OrganizerTag` values as tag sources
5. **Venue mapping:** Match EventBrite venues to existing Ticketmaster venues by lat/lon + name similarity, create new venue records for unmatched
6. **Full details:** Hit `/v3/events/{id}/?expand=venue,organizer` for events worth importing

### Rate Budget Estimate

- Discovery: ~200 pages × 50 events = 10,000 events in ~200 calls
- Detail fetch: ~2,000 music events × 1 call each = ~2,000 calls
- Total: ~2,200 calls per full sync (well within 48,000/day limit)
- Can run a full sync every few hours without hitting limits
