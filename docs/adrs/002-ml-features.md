# ADR 002: ML Features Brainstorm

## Context

Vibe Seeker matches users' music taste (Spotify → Last.fm tags → weighted vibe vectors) to NYC venues (Ticketmaster events → artist tags → venue vibe vectors) using cosine similarity. The system currently has **zero ML** — it's all weighted aggregation + vector math. The gap: Ticketmaster only covers ~55 active venues and misses small/DIY spaces entirely. Expanding to less-structured data sources (EventBrite, venue websites, social media) is a key motivation.

**EventBrite API status:** Explored 2026-04-02 with live API calls. See `docs/eventbrite.md` for full findings. Key discovery: data is **much more structured than assumed** — artist names are in event titles, rich typed tags provide genre data, and a working discovery endpoint exists.

---

## Category A: Unstructured Data → Structured Events

The core problem: the most interesting venues aren't on well-structured APIs. These features all address "how do we get usable event data from messy sources?"

> **Update (2026-04-02):** EventBrite API research revealed that their data is semi-structured, not fully unstructured. Artist names are in event titles (comma-separated), events have typed category/subcategory/format tags, and organizer-defined tags often include granular genre info like "Indierock", "Electropop". This reduces the ML difficulty for A1/A2 but doesn't eliminate it — edge cases and miscategorized events still exist. See `docs/eventbrite.md` for details.

### A1. Named Entity Recognition (NER) for Artist Extraction

**What it is:** Extract artist names from event titles and descriptions.

**Real EventBrite examples (from API research):**

| Title | Artists | Difficulty |
|-------|---------|-----------|
| `Six Sex, deBasement` | Six Sex, deBasement | Easy — comma-separated |
| `Frog, Olivia O., DJ Silky Smooth` | Frog, Olivia O., DJ Silky Smooth | Easy |
| `Elsewhere Presents: Pearly Drops, Teather @ Market Hotel` | Pearly Drops, Teather | Medium — prefix & venue in title |
| `SOLD OUT - Jesse Malin + John Varvatos: Almost Grown` | None (book event, not music) | Hard — looks like artists but isn't |
| `MALIE DONN LIVE IN NYC` | Malie Donn | Medium — all caps, no comma separation |
| `LOVE 2 LOVE - A DONNA SUMMER DISCO PARTY` | None (tribute/themed party) | Hard — references artist who isn't performing |

**Approaches (spectrum from simple to complex):**

| Approach | ML Concepts Learned | Portfolio Signal | Effort |
|----------|-------------------|-----------------|--------|
| **Regex/heuristics** (comma split, strip prefixes) | None (baseline) | Low | Low |
| **LLM API extraction** (Claude/GPT) | Prompt engineering, structured output | Low-medium | Low |
| **Pre-trained NER** (spaCy, Hugging Face) | Transfer learning, tokenization, entity types | Medium | Medium |
| **Fine-tuned NER model** | Training loops, labeled data, evaluation metrics (F1, precision/recall), overfitting | High | High |

**Why music NER is still interesting even with semi-structured data:**
- The easy cases (comma-separated concert titles) are ~70% of music events — regex handles these
- The remaining 30% are genuinely hard: "Beach House", "The National", "Cake", "!!!" break NER assumptions
- Multi-word entities with ambiguous boundaries: "DJ Shadow and Cut Chemist" — is that one act or two?
- Distinguishing performing artists from referenced artists (Donna Summer tribute) or non-music people (authors at book events)
- A model that handles the full range is portfolio-worthy, and the easy cases provide training signal

**The learning-optimized path:**
1. Build regex baseline — comma/`+` split on concert-format titles. Measure precision/recall.
2. Use LLM API calls as a **labeling tool** — run Claude over 500+ EventBrite titles to generate training data
3. Fine-tune a smaller model (DistilBERT, spaCy) on the labeled data
4. Compare all three approaches and document the progression
5. This teaches the full ML lifecycle: data collection → labeling → training → evaluation → deployment

**ML concepts you'd learn:** NER, tokenization, BIO tagging scheme, transfer learning, fine-tuning, evaluation metrics (precision/recall/F1), data annotation, model comparison

### A2. Event Classification (Music vs Non-Music, Event Type)

**What it is:** Classify events by type from their title + description + tags.

**Classes:** `music_concert | dj_night | comedy | theater | art | community | sports | other`

**Update based on API research:** EventBrite's `EventbriteCategory` and `EventbriteFormat` tags handle ~90% of classification:
- `EventbriteCategory/103` = Music
- `EventbriteFormat/6` = Concert or Performance
- `EventbriteFormat/11` = Party or Social Gathering

**Where ML is still needed:** Miscategorized events. Real examples:
- "MALIE DONN LIVE IN NYC" categorized as Seasonal & Holiday (should be Music)
- "Indian Speed Dating" has tags including "Bollywoodmusic" but isn't a music event
- Book events at Strand Book Store use `+` between author names, mimicking concert title format

**Approaches:**

| Approach | ML Concepts | Portfolio | Effort |
|----------|------------|-----------|--------|
| **Tag-based rules** (EventbriteCategory filter) | None (baseline) | Low | Low |
| **TF-IDF + Logistic Regression** on title+description+tags | Feature engineering, text vectorization, linear classifiers | Medium | Low-medium |
| **Fine-tuned BERT classifier** | Transformers, fine-tuning, attention, GPU training | High | Medium-high |
| **Zero-shot classification** (Hugging Face pipeline) | Inference-only, no training | Medium | Low |

**The learning-optimized path:**
1. Start with tag-based rules as baseline — measure how many events the category tags correctly classify
2. Build TF-IDF + Logistic Regression for edge cases — teaches text classification fundamentals
3. Compare: rules vs ML vs combined approach
4. The comparison itself is portfolio gold: "I evaluated multiple approaches and chose X because Y"

**ML concepts you'd learn:** Text preprocessing, TF-IDF, bag-of-words, logistic regression, cross-validation, confusion matrices, feature engineering with structured + unstructured data

### A3. EventBrite Integration + Extraction Pipeline

**What it is:** Build the data pipeline that connects EventBrite to the existing vibe computation system.

**Updated architecture (based on API research):**
```
POST /v3/destination/search/ (NYC, paginate)
  → Filter: EventbriteCategory/103 (Music) in tags
  → GET /v3/events/{id}/?expand=venue (for detail)
  → Branch by format:
    ├─ Concert (Format 6): Parse title for artist names → Last.fm tag pipeline
    ├─ Party (Format 11): Use OrganizerTags as direct tag source (lower weight)
    └─ Other: Skip or low-weight OrganizerTag contribution
  → Venue dedup: Match to existing TM venues by lat/lon + name similarity
  → Upsert to existing schema with data_source="eventbrite"
```

**This is primarily engineering**, but it's the delivery vehicle for A1 and A2. The ML models handle the edge cases that structured parsing can't.

**New opportunity from API research:** EventBrite's `OrganizerTag` values (Indierock, Electropop, Hyperpop, etc.) could be used directly as tag sources — potentially supplementing or replacing Last.fm lookups for artists found only on EventBrite. These tags would need normalization to align with the existing Last.fm tag vocabulary.

---

## Category B: Smarter Matching

The core problem: current matching is good but treats tags as independent, unrelated dimensions. These features make the matching itself more intelligent.

### B1. Tag Embeddings (Co-occurrence Based)

**What it is:** Learn dense vector representations of tags from your existing data, so semantically similar tags (shoegaze ↔ dream pop) are close in vector space.

**Why it matters:** Currently, a user who loves "shoegaze" scores 0.0 against a venue that books "dream pop" — even though they'd probably love it. Tags are treated as independent dimensions with no semantic relationship.

**The approach (no external APIs needed):**
1. Build a tag co-occurrence matrix from `artist_tags`: two tags co-occur when they appear on the same artist
2. You already have this data — ~2000 unique tags across ~1200 artists
3. Apply SVD (Singular Value Decomposition) to reduce to ~50-100 dimensions
4. Now each tag is a dense vector; similar tags are nearby
5. Replace (or blend with) sparse cosine similarity using dense cosine similarity

**Alternatively:** Use Word2Vec/GloVe-style training on "sentences" of tags per artist. Each artist's tag list is treated as a "document," and you learn embeddings that capture co-occurrence patterns.

**Why this is a great first ML project:**
- Uses data you already have (zero new data collection)
- The math is fundamental (SVD, matrix factorization, cosine similarity — you already use cosine similarity!)
- Results are immediately visible in the app (match scores change)
- Can be implemented in Go (gonum library) or Python (scikit-learn, one function call)
- Easy to A/B compare: old sparse matching vs. new dense matching

**ML concepts you'd learn:** Embeddings, dimensionality reduction, SVD/PCA, Word2Vec, vector spaces, the fundamental idea that "similar things are nearby in learned vector space"

### B2. Co-Billing Graph Analysis (Label Propagation)

**What it is:** Infer tags for unknown artists based on who they perform with.

**The insight:** Your `show_artists` table is a bipartite graph (artists ↔ shows). If Artist X (no tags) always co-bills with indie rock acts, they're almost certainly indie-adjacent.

**Approaches:**
1. **Simple SQL aggregation** (not really ML, but a good baseline):
   ```sql
   SELECT at.tag, AVG(at.count) as avg_relevance
   FROM show_artists sa1
   JOIN show_artists sa2 ON sa1.show_id = sa2.show_id
     AND sa1.artist_id != sa2.artist_id
   JOIN artist_tags at ON at.artist_name = sa2.artist_id
   WHERE sa1.artist_id = 'unknown-artist-slug'
   GROUP BY at.tag ORDER BY avg_relevance DESC;
   ```

2. **Label propagation on graph** — iteratively spread tag information through the artist-show graph until convergence. This is a real graph ML algorithm.

3. **Graph neural network (GNN)** — learn node embeddings in the artist-show-venue graph. Overkill for your data size but impressive on a portfolio.

**ML concepts you'd learn:** Graph theory, bipartite graphs, label propagation, (optionally) graph neural networks, semi-supervised learning

### B3. Learned User Preference Weights

**What it is:** Instead of fixed weights (medium_term = 1.0×, short_term = 0.5×, recency buckets), learn optimal weights from user feedback.

**Requires:** A feedback mechanism (likes, saves, "I went to this show", star ratings).

**Approach:** Treat it as a learning-to-rank problem. Given a user's vibe profile and a set of venues, learn weight parameters that best predict which venues the user actually engages with.

**ML concepts:** Learning to rank, gradient descent, loss functions, feature importance

---

## Category C: Discovery & Exploration Features

These create new UX capabilities beyond "here's your match score."

### C1. Venue Clustering / Scene Detection

**What it is:** Group venues into "scenes" based on their vibe profiles + geography.

**Example output:** "Brooklyn Noise Scene" (cluster of 5 venues in Bushwick/Williamsburg that all book noise/experimental/drone), "East Village Jazz" (cluster of 3 venues with jazz/bebop/improvisation profiles)

**Approach:**
- Feature vector per venue: vibe profile (tag weights) + lat/lon coordinates
- K-means, DBSCAN, or hierarchical clustering
- Auto-label clusters by dominant tags + neighborhood
- Surface in UI as an exploration mode

**ML concepts you'd learn:** Clustering (unsupervised learning), k-means, DBSCAN, silhouette scores, feature scaling, the unsupervised vs. supervised distinction

### C2. Temporal Trend Detection

**What it is:** Track how venue/neighborhood vibes shift over time.

**Example insight:** "Bushwick venues have shifted from indie rock toward electronic/experimental over the past 6 months."

**Requires:** Storing historical vibe snapshots (currently overwritten on each sync).

**Approach:**
- Store timestamped venue vibe snapshots
- Time series analysis on tag weights per venue/neighborhood
- Change point detection or simple moving averages
- Visualization: animated map or trend charts

**ML concepts:** Time series analysis, change point detection, moving averages, temporal feature engineering

### C3. Venue Image Classification

**What it is:** Classify venue photos to infer vibe from visual aesthetics.

**Example:** Dim lighting + exposed brick → "intimate jazz bar" vibe. Neon lights + large floor → "dance club" vibe.

**Approach:**
- Ticketmaster provides `image_url` for venues
- Fine-tune a CNN (ResNet) or use CLIP zero-shot classification
- Add visual vibe signals as additional features in venue profiles

**ML concepts you'd learn:** Computer vision, CNNs, transfer learning, CLIP, multi-modal features

**Portfolio angle:** Multi-modal ML (text + images) is a strong signal. Most portfolio projects only use one modality.

### C4. Natural Language Venue Search

**What it is:** "Find me a chill jazz bar in Brooklyn with cheap drinks" → ranked venue results.

**Approach:**
- Encode the query and venue profiles into a shared embedding space
- Rank by similarity
- Could use sentence-transformers for encoding, or an LLM for query understanding + your existing matching

**ML concepts:** Semantic search, sentence embeddings, query understanding, information retrieval

---

## Category D: Data Quality & Operations

### D1. Fuzzy Entity Deduplication

**What it is:** Same artist/venue appears under different names across data sources. "The Black Keys" vs "Black Keys" vs "the black keys". Or same event listed on Ticketmaster and EventBrite.

**Approach:**
- String similarity (Levenshtein, Jaro-Winkler)
- Phonetic matching (Soundex, Metaphone)
- Learned string similarity (Siamese networks)
- Blocking + pairwise comparison for efficiency

**ML concepts:** Record linkage, string similarity metrics, blocking strategies, (optionally) siamese neural networks

### D2. Data Quality Scoring

**What it is:** Automatically flag suspicious data — mis-geocoded venues, defunct venues listed as active, non-music venues in music results.

**Approach:** Anomaly detection on venue features (location, event frequency, description text, classifications).

**ML concepts:** Anomaly detection, feature engineering, rule-based vs. statistical approaches

---

## ML vs Non-ML: Honest Assessment

### Genuinely need ML (no good non-ML alternative)

| Feature | Why ML is necessary |
|---------|-------------------|
| **B1. Tag Embeddings** | ~2000 unique tags. A manual synonym table would be incomplete and unmaintainable. SVD/embeddings naturally discover that "shoegaze" and "dream pop" are related from co-occurrence. |
| **C1. Venue Clustering** | High-dimensional vibe vectors + geography. Manual grouping wouldn't capture cross-neighborhood scenes or subtle vibe similarities. |
| **C3. Image Classification** | Extracting vibe from photos is inherently a perception task. No rule-based approach works. |

### ML improves on a strong non-ML baseline (nice-to-have)

| Feature | Non-ML baseline | What ML adds | Non-ML accuracy |
|---------|----------------|-------------|-----------------|
| **A1. Artist NER** | Comma/`+` split, strip prefixes | Edge cases: all-caps, tributes, ambiguous `+` | ~70-80% |
| **A2. Event Classification** | `EventbriteCategory/103` tag filter | Miscategorized events | ~90% |
| **C4. NL Search** | Keyword matching + tag filter | Semantic: "chill" → jazz/acoustic/lo-fi | Exact matches only |

### Non-ML is sufficient (ML would be over-engineering)

| Feature | Recommended approach |
|---------|---------------------|
| **A3. EventBrite Integration** | Go API client (same pattern as `backend/internal/ticketmaster/`) |
| **B2. Co-billing Inference** | SQL aggregation — the query is 6 lines and solves the problem |
| **B3. Learned Weights** | No feedback data yet. Grid search when data exists. |
| **C2. Trend Detection** | Moving averages + percentage change. Add timestamped vibe snapshots to DB. |
| **D1. Fuzzy Deduplication** | Levenshtein / Jaro-Winkler string similarity with blocking |
| **D2. Data Quality Scoring** | Rule-based flags (no events in 12mo = defunct, lat/lon outside NYC = bad geocoding) |

**Summary:** 3 genuinely need ML, 3 benefit from ML over a baseline, 6 are better solved with conventional engineering.

---

## Dependency Graph

```
Engineering track (ship features):      ML learning track (portfolio):
  A3. EventBrite API client               B1. Tag Embeddings (existing data)
  B2. Co-billing SQL query                C1. Venue Clustering (existing data)
  D1. Fuzzy dedup (when 2 sources)        A1. NER comparison study (needs A3 data)
  C2. Vibe snapshots (schema change)      C3. Image classification (optional)
```

Both tracks can proceed in parallel. Within each track, items are listed in recommended order.

---

## Portfolio Impact Assessment

| Feature | Needs ML? | Visible in App | "Wow Factor" | Good First Project? |
|---------|-----------|----------------|-------------|-------------------|
| **B1. Tag Embeddings** | Yes | Yes (scores change) | Medium | **Best first ML project** |
| **C1. Venue Clustering** | Yes | Yes (new UI) | High | Good after B1 |
| **A1. Music NER** | Improves baseline | Yes (more venues) | High | Good comparison study |
| **A3. EventBrite Integration** | No (engineering) | Yes (more venues) | Medium | **Best first engineering project** |
| **B2. Co-billing Inference** | No (SQL) | Yes (fewer unknowns) | Medium | Quick win |
| **D1. Fuzzy Dedup** | No (algorithms) | Indirect | Low | Needed for multi-source |
| **C3. Image Classification** | Yes | Yes (visual) | High | Flashy but tangential |
| **C4. NL Search** | Improves baseline | Yes (new feature) | High | Needs B1 first |

---

## Recommended Path (Both Tracks)

### Phase 1 (parallel):
- **Engineering:** A3 (EventBrite client) — doubles venue coverage
- **ML:** B1 (Tag Embeddings) — learns the most fundamental ML concept

### Phase 2 (parallel):
- **Engineering:** B2 (co-billing SQL) + D1 (fuzzy dedup for TM↔EB venues)
- **ML:** C1 (Venue Clustering) — unsupervised learning, new UI feature

### Phase 3:
- **ML:** A1 (Artist NER comparison study) — build regex baseline on A3 data, then beat it with ML
- **Engineering:** C2 (vibe snapshots schema) — prep for future trend analysis

### Phase 4 (interest-driven):
- C4 (NL Search), C3 (Image Classification), A2 (Event Classification ML layer)

---

## Language/Stack Considerations

| Tool | Best For | Notes |
|------|----------|-------|
| **Python** (scikit-learn, PyTorch, Hugging Face) | Model training, prototyping, NLP | ML ecosystem. Use for training and experimentation. |
| **Go** (gonum, existing backend) | Inference, pipelines, API serving | Production. Also for A3, B2, D1. |
| **Python microservice** | Serving trained models | Simple HTTP service alongside Go backend. |
| **LLM APIs** (Claude, etc.) | Labeling data, baselines | Generate training data for A1, comparison baselines. |

**Practical suggestion:** Prototype ML in Python (Jupyter + scikit-learn). Precompute embeddings/clusters in Python, store in Postgres. Engineering work stays in Go.

---

## Next Steps

1. **Engineering:** Start A3 — build EventBrite Go API client (see `docs/eventbrite.md`)
2. **ML:** Start B1 — prototype tag embeddings in Jupyter with existing `artist_tags` data
3. **Setup:** Python ML environment (Jupyter, scikit-learn, pandas, numpy)
4. **After Phase 1:** B2 (SQL) + D1 (dedup) + C1 (clustering)
