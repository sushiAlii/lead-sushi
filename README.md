# lead-sushi

A small Go CLI that finds local businesses worth pitching a website to.

It searches Google Places for a niche in a city, then does one of two things:

- **`--target weaksite`** (default): fetches each business's homepage and scores how weak it is. The weakest sites come first in the CSV, each with a plain-language list of problems you can quote in an outreach message.
- **`--target nosite`**: keeps businesses that have reviews but no website, or only a Facebook, Instagram or directory page.

Results are written to a CSV. Nothing is sent to anyone.

## Install

Requires Go 1.27 or newer.

```
git clone https://github.com/sushiAlii/lead-sushi
cd lead-sushi
make build
```

## Setup

1. Create a Google Cloud project and enable **Places API (New)**.
2. Create an API key and restrict it to **Places API (New)** only.
3. Set a budget alert in Billing before your first run.
4. Export the key:

```
export GOOGLE_PLACES_API_KEY="your-key"
```

## Usage

```
make run ARGS="--city sydney --niche clinics --max-requests 15"
```

Or call the binary directly:

```
bin/lead-sushi --city melbourne --niche beauty --min-score 6 --out beauty.csv
bin/lead-sushi --city perth --terms "dentist,orthodontist" --max-requests 30
bin/lead-sushi --city sydney --niche trades --target nosite --out nosite.csv
```

| Flag | Default | Meaning |
|---|---|---|
| `--city` | `sydney` | `sydney`, `melbourne`, `brisbane`, `perth` |
| `--niche` | `clinics` | `clinics`, `beauty`, `trades`, `auto`, `pro` |
| `--terms` | | Comma-separated search terms, overrides `--niche` |
| `--target` | `weaksite` | `weaksite` or `nosite` |
| `--min-reviews` | `10` | Minimum Google review count |
| `--min-rating` | `4.0` | Minimum Google rating |
| `--min-score` | `5` | Minimum weakness score to keep (`weaksite` only) |
| `--max-requests` | `60` | Hard cap on Places API requests for the whole run |
| `--workers` | `10` | Concurrent website audits |
| `--out` | `leads.csv` | Output path |

Each query prints why results were rejected, for example `map[ok:2 wrong_site_type:17 few_reviews:1]`, so you can see which filter to loosen.

## Cost

Each query makes up to 3 Places requests (60 results). Website and phone fields are billed at Google's higher Enterprise rate, so start small with `--max-requests 15` and check your billing page. The audit step only makes plain HTTP requests to the businesses' own sites and costs nothing.

## How the weakness score works

Each failed check adds points. A site is kept when its total reaches `--min-score`.

| Check | Points |
|---|---|
| Broken (unreachable, or HTTP 4xx/5xx) | 5 |
| No `viewport` meta tag (not mobile friendly) | 3 |
| No call to action (no `tel:` link, form, or book/contact link) | 3 |
| Not served over HTTPS | 2 |
| Slow (over 3 seconds) | 2 |
| Outdated (copyright year 2019 or earlier, or `<font>`, `<marquee>`, `<blink>`) | 2 |
| Built on Wix, Weebly, Yola, Squarespace or GoDaddy | 2 |
| Missing title, meta description or `<h1>` | 1 |

Sites that answer with 403, 429 or 503 are treated as blocking bots, not as broken, and are skipped.

The checks read only the homepage HTML. A site that renders everything with JavaScript can look emptier than it is, so open your top results before you pitch.

## Output

`name, phone, address, rating, reviews, website_status, website, weak_score, problems, maps_url, place_id`, sorted by `weak_score` and then review count.

## Development

```
make check    # gofmt check, go vet, tests with the race detector
make build
```

Only one dependency: `golang.org/x/net/html`.

## Responsible use

- Australia's Spam Act 2003 covers commercial email and SMS. Messages must be relevant to the recipient's business, identify the sender, and include a working way to opt out.
- Respect Google Maps Platform's terms, including limits on storing and caching Places data.
- Audits make one polite request per site with an identifying `User-Agent`. Keep `--workers` modest.
- Lead CSVs contain business contact details. They are git-ignored, so don't commit them.

## License

[MIT](LICENSE)
