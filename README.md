# inspect

**Point, highlight, report.** Inspect is a small tool you embed in a web app. A user screenshots or records the broken part of a page, marks it up, writes a title, and a GitHub issue is created with the media and the technical context attached.

```
 browser (any web app)                        inspect server (Go)                 GitHub
┌──────────────────────────┐  multipart   ┌────────────────────────┐   REST   ┌──────────────────┐
│ @uug-ai/inspect          │ ───────────▶ │ validate + sniff media │ ───────▶ │ assets branch    │
│ area / element / record  │  payload +   │ store screenshot/video │          │ (orphan, media)  │
│ annotate, describe       │  attachments │ render issue markdown  │ ───────▶ │ issue            │
└──────────────────────────┘ ◀─────────── └────────────────────────┘          └──────────────────┘
                               {number,url}      holds the GitHub token
```

| Package | What it is |
|---|---|
| [`client/`](client) | `@uug-ai/inspect`: the browser library (TypeScript, no framework, Shadow DOM). |
| [`server/`](server) | The Go service that keeps the GitHub token, stores the media and creates the issue. |

## Try it locally

```sh
# 1. server in dry-run mode: no GitHub calls, media on disk, issue previews served locally
cd server
INSPECT_DRY_RUN=true INSPECT_STORE=local INSPECT_DEFAULT_REPO=uug-ai/inspect \
INSPECT_ALLOWED_ORIGINS=http://localhost:5180 go run .

# 2. demo app
cd client && npm install && npm run dev
# open http://localhost:5180/?endpoint=http://localhost:8080
```

Once a report is submitted, the toast links to a preview of the markdown issue that would have been created.
