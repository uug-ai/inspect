# inspect server

The backend for the inspect browser library. A user selects a region, picks an
element or records part of a web app, annotates it and writes a title and
description. The browser POSTs that report here. The server:

1. validates the payload and sniffs every attachment's real media type,
2. stores the media where GitHub can render it (by default it commits to an
   orphan `inspect-assets` branch),
3. opens a GitHub issue with the description, the media and a collapsed
   *Context* section.

The GitHub token stays on the server. The browser never sees it, which is the
reason this service exists.

## API

### `POST /v1/issues`

The request is `multipart/form-data` with these fields:

| Field        | Description |
|--------------|-------------|
| `payload`    | JSON (below). Required. |
| `attachment` | 0 to 5 files. Allowed types: PNG, JPEG, WebP, WebM and MP4. The server sniffs the type from the content and ignores the client-declared type. |

```json
{
  "title": "required, 1..256 chars",
  "description": "markdown, optional, max 20000 chars",
  "kind": "screenshot | recording | note",
  "repo": "owner/name (optional, must be in INSPECT_ALLOWED_REPOS)",
  "labels": ["optional; only INSPECT_ALLOWED_LABELS pass through"],
  "context": {
    "url": "...", "title": "...", "userAgent": "...",
    "viewport": {"width": 1440, "height": 900, "dpr": 2},
    "timestamp": "RFC3339",
    "selector": "...", "region": {"x": 0, "y": 0, "width": 100, "height": 100},
    "console": [{"level": "error", "message": "...", "time": "RFC3339"}],
    "app": {"version": "1.2.3"}
  }
}
```

Every `context` field is optional, and `context` itself may be left out. If the
user turns off technical context, the client sends only `{"timestamp": ...}`. In
that case the issue has no Context section. Rows for empty fields are always
left out. Over-long context fields are truncated rather than rejected. Only the
50 most recent console entries are kept. Unknown JSON fields are ignored.

- If API keys are configured, send `Authorization: Bearer <key>`.
- On success the server returns `201 {"number": 123, "url": "https://github.com/o/r/issues/123"}`.
- On error it returns `{"error": "message"}` with one of these statuses: 400 (validation), 401 (auth), 403 (origin), 413 (too large), 415 (media type), 429 (rate limit, with `Retry-After`) or 502 (GitHub failure; details are logged, not returned).

### Other endpoints

- `OPTIONS /v1/issues` is the CORS preflight.
- `GET /healthz` returns `200 ok`.
- `GET /assets/...` serves stored files. It exists only when `INSPECT_STORE=local`.
- `GET /preview/{id}` returns the would-be issue as plain-text markdown. It exists only when `INSPECT_DRY_RUN=true`, and the last 100 previews are kept in memory.

## Configuration

| Variable | Default | Description |
|---|---|---|
| `GITHUB_TOKEN` | (none) | Token used for all GitHub calls. Required, except when `INSPECT_STORE=local` and `INSPECT_DRY_RUN=true`. |
| `INSPECT_DEFAULT_REPO` | (none, required) | `owner/name` used when the payload has no `repo`. |
| `INSPECT_ALLOWED_REPOS` | the default repo | Comma-separated repos a client may target. The default repo is always allowed. If the payload names any other repo, the server returns 400. |
| `INSPECT_ASSETS_REPO` | the issue's repo | Repo that stores the media. |
| `INSPECT_ASSETS_BRANCH` | `inspect-assets` | Branch that stores the media. If it is missing, the server creates it as an orphan branch. |
| `INSPECT_ALLOWED_ORIGINS` | (empty) | Comma-separated browser origins, or `*`. If empty, every request that carries an `Origin` header is rejected. |
| `INSPECT_API_KEYS` | (empty) | Comma-separated bearer keys. If empty, no auth is required. |
| `INSPECT_LABELS` | `inspect` | Labels added to every issue. |
| `INSPECT_ALLOWED_LABELS` | (empty) | Client labels that may pass through. If empty, all client labels are dropped. |
| `INSPECT_MAX_UPLOAD_MB` | `50` | Limit on the total size of all attachments. |
| `INSPECT_MAX_FILE_MB` | `INSPECT_MAX_UPLOAD_MB` | Limit for each attachment. It is capped at the total limit. |
| `INSPECT_RATE_LIMIT` | `10` | `POST`s per minute per client IP, with a burst of the same size. `0` turns rate limiting off. |
| `INSPECT_TRUST_PROXY` | `false` | Set this when the service runs behind a reverse proxy. The client IP is then taken from the right-most `X-Forwarded-For` entry. |
| `INSPECT_STORE` | `github` | `github` or `local`. |
| `INSPECT_LOCAL_DIR` | `./data` | Directory used by the local store. |
| `INSPECT_DRY_RUN` | `false` | Skips issue creation. The would-be issue is logged and the response is `{"number":0,"url":"<public url>/preview/<id>"}`. Media still goes to the configured store, so use `INSPECT_STORE=local` to avoid touching GitHub at all. |
| `INSPECT_PUBLIC_URL` | `http://localhost:$PORT` | Base URL of this service, used for local asset URLs and preview URLs. |
| `GITHUB_API_URL` | `https://api.github.com` | REST API base URL, for GitHub Enterprise or tests. |
| `PORT` | `8080` | Listen port. |

### Token scopes

The recommended token is a fine-grained personal access token (or, later, a
GitHub App) with access to the issue repos and the assets repo. It needs:

- **Issues: Read and write** on every repo in `INSPECT_ALLOWED_REPOS`.
- **Contents: Read and write** on the assets repo. The server uses it to create the orphan branch with the Git Data API and to commit files with the Contents API.
- **Metadata: Read**, which GitHub grants automatically.

To use a classic PAT instead, give it `repo` scope for private repos or
`public_repo` for public ones.

The token is read through the `github.TokenProvider` interface.
`StaticToken` is the only implementation today. GitHub App installation tokens
can be added later behind the same interface.

## How media is stored

Files are committed to `INSPECT_ASSETS_BRANCH` of the assets repo at
`inspect/YYYY/MM/DD/<random-id>/screenshot-N.<ext>` or `recording-N.<ext>`.
If that branch does not exist, the server creates it as an orphan branch:
blob (README) → tree → commit with no parents → ref. The branch therefore
shares no history with the main code. Retries cover concurrent commits from
several replicas (409) and a deleted branch (404).

In the issue body:

- Images are embedded as `![screenshot-1.png](https://github.com/o/r/blob/<branch>/<path>?raw=true)`.
- Videos are linked: the URL on its own line, followed by `[▶ recording-1.webm](...)`. GitHub only plays inline videos that were uploaded through its own UI.

People who view the issue need read access to the assets repo to see the media.
If the assets repo is private, the media stays private as well.

Storage sits behind `store.Store` (`Put(ctx, name, contentType, data) (url, error)`),
so an S3 or MinIO store can be added later.

## Safety measures

- **CORS and origin check.** A multipart POST is a "simple" request, so browsers send it without a preflight. The server therefore also rejects a disallowed `Origin` itself instead of relying on the browser.
- **Mentions.** In user-provided text, each `@` that could start a GitHub mention (at the start of the text or after a non-word character, followed by a letter or digit) gets a zero-width joiner (U+200D) after it. This stops anonymous reporters from pinging users or teams. It looks the same once rendered. The trade-off is that copied text, including code inside the description, contains an invisible extra character. Email addresses such as `a@b.com` are left alone.
- **Tables and code fences.** User text in table cells is collapsed to one line, and `|`, `` ` ``, `\`, `<` and `>` are escaped, so it cannot break the table or close `<details>`. Each console fence is longer than the longest run of backticks in its content.
- **Labels.** Client labels are dropped unless they are listed in `INSPECT_ALLOWED_LABELS`.
- **Size limits.** `http.MaxBytesReader` caps the whole body. Each part is read with its own limit, and the server has read, write and idle timeouts. Outbound GitHub calls time out after 2 minutes.
- **API keys.** A key embedded in browser JavaScript is public. It only deters casual abuse. The origin allowlist and the rate limit do the real work.

## Run locally against GitHub

`go run .` loads `.env` (committed defaults, no secrets) and then `.env.local` (gitignored). Variables already set in the environment take precedence over both files, and the files are never copied into the Docker image.

1. Create a [fine-grained token](https://github.com/settings/personal-access-tokens/new). Set the resource owner to `uug-ai` and give it access only to the test repo, with **Issues: read and write** and **Contents: read and write**.
2. Put it in `server/.env.local` as `GITHUB_TOKEN=…`. To send test issues to a private sandbox repo, override `INSPECT_DEFAULT_REPO` and `INSPECT_ALLOWED_REPOS` there as well.
3. Run `go run .` in `server/`, and `npm run dev` in `client/`. Then open `http://localhost:5180/?endpoint=http://localhost:8080`.
   In VS Code, the compound launch **Stack: Inspect demo + server (GitHub)** starts both.

The first report creates the orphan `inspect-assets` branch in the repo. After that, each report adds one commit for its media and one issue labelled `inspect`.

## Run locally (dry run, no GitHub)

```sh
cd server
INSPECT_DRY_RUN=true INSPECT_STORE=local \
INSPECT_DEFAULT_REPO=uug-ai/hub-frontend \
INSPECT_ALLOWED_ORIGINS=http://localhost:4200 \
go run .
```

```sh
curl -s -X POST http://localhost:8080/v1/issues \
  -F 'payload={"title":"Save button does nothing","kind":"screenshot","description":"Clicked save.","context":{"url":"http://localhost:4200/settings","timestamp":"2026-11-12T10:00:00Z"}}' \
  -F 'attachment=@screenshot.png'
# {"number":0,"url":"http://localhost:8080/preview/3f1c..."}

curl -s http://localhost:8080/preview/3f1c...   # the rendered issue markdown
```

To create real issues, drop `INSPECT_DRY_RUN` and `INSPECT_STORE`, then set
`GITHUB_TOKEN`.

## Development

```sh
go vet ./...
go test ./...
docker build -t inspect-server .
```

The layout is:

- `main.go` handles wiring and graceful shutdown.
- `internal/config` reads the environment.
- `internal/github` is a minimal REST client.
- `internal/store` holds the GitHub-branch and local stores.
- `internal/issue` covers validation, sniffing, body rendering and submission.
- `internal/httpapi` covers HTTP, CORS, auth and rate limiting.
