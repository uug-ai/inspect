import type { AssistDraft, AssistRequest, InspectOptions, RepoOptions, Report, SubmitResult } from './types';

/** POST the report to the inspect server as multipart/form-data. */
export async function submitReport(options: InspectOptions, report: Report): Promise<SubmitResult> {
  if (options.onSubmit) return options.onSubmit(report);
  if (!options.endpoint) throw new Error('Inspect is not configured: set `endpoint` or `onSubmit`.');

  const body = new FormData();
  body.append('payload', JSON.stringify(report.payload));
  for (const file of report.attachments) body.append('attachment', file, file.name);

  const headers = options.headers ? await options.headers() : {};
  let res: Response;
  try {
    res = await fetch(`${options.endpoint.replace(/\/+$/, '')}/v1/issues`, { method: 'POST', body, headers });
  } catch {
    throw new Error('Could not reach the issue server. Check your connection and try again.');
  }

  let data: { error?: string; number?: number; url?: string } = {};
  try {
    data = await res.json();
  } catch {
    // Non-JSON body (proxy error page, etc.) — fall through to the status message.
  }
  if (!res.ok) {
    throw new Error(data.error || `The issue server returned ${res.status}.`);
  }
  return { number: data.number, url: data.url };
}

function baseUrl(options: InspectOptions): string {
  return (options.endpoint ?? '').replace(/\/+$/, '');
}

const capabilities = new Map<string, Promise<{ assist: boolean }>>();

/** Which optional features the server has on. Cached per endpoint; failures mean "none". */
export function serverCapabilities(options: InspectOptions): Promise<{ assist: boolean }> {
  const base = baseUrl(options);
  if (!base) return Promise.resolve({ assist: false });
  let p = capabilities.get(base);
  if (!p) {
    p = (async () => {
      try {
        const controller = new AbortController();
        const timer = setTimeout(() => controller.abort(), 4000);
        const headers = options.headers ? await options.headers() : {};
        const res = await fetch(`${base}/v1/capabilities`, { headers, signal: controller.signal });
        clearTimeout(timer);
        if (!res.ok) return { assist: false };
        const data = (await res.json()) as { assist?: boolean };
        return { assist: data.assist === true };
      } catch {
        return { assist: false };
      }
    })();
    capabilities.set(base, p);
  }
  return p;
}

/** Ask the inspect server's writing assistant for a title and description. */
export async function requestAssist(options: InspectOptions, request: AssistRequest): Promise<AssistDraft> {
  if (typeof options.onAssist === 'function') return options.onAssist(request);
  const body = new FormData();
  body.append(
    'payload',
    JSON.stringify({ title: request.title, description: request.description, kind: request.kind, context: request.context }),
  );
  for (const image of request.images) body.append('attachment', image, image.name);
  const headers = options.headers ? await options.headers() : {};
  let res: Response;
  try {
    res = await fetch(`${baseUrl(options)}/v1/assist`, { method: 'POST', body, headers });
  } catch {
    throw new Error('Could not reach the writing assistant. Check your connection and try again.');
  }
  let data: { error?: string; title?: string; description?: string } = {};
  try {
    data = await res.json();
  } catch {
    // fall through
  }
  if (!res.ok || !data.title || !data.description) {
    throw new Error(data.error || `The writing assistant returned ${res.status}.`);
  }
  return { title: data.title, description: data.description };
}

const EMPTY: RepoOptions = { labels: [], assignees: [] };
const repoOptionsCache = new Map<string, { at: number; value: Promise<RepoOptions> }>();
const OPTIONS_TTL = 5 * 60 * 1000;

/** The target repo's labels and assignable people (cached 5 minutes). Failures mean "none". */
export function fetchRepoOptions(options: InspectOptions): Promise<RepoOptions> {
  if (options.onOptions === false) return Promise.resolve(EMPTY);
  if (typeof options.onOptions === 'function') return options.onOptions().catch(() => EMPTY);
  const base = baseUrl(options);
  if (!base) return Promise.resolve(EMPTY);
  const key = `${base}|${options.repo ?? ''}`;
  const cached = repoOptionsCache.get(key);
  if (cached && Date.now() - cached.at < OPTIONS_TTL) return cached.value;
  const value = (async (): Promise<RepoOptions> => {
    try {
      const headers = options.headers ? await options.headers() : {};
      const query = options.repo ? `?repo=${encodeURIComponent(options.repo)}` : '';
      const res = await fetch(`${base}/v1/options${query}`, { headers });
      if (!res.ok) return EMPTY;
      const data = (await res.json()) as Partial<RepoOptions>;
      return { labels: data.labels ?? [], assignees: data.assignees ?? [] };
    } catch {
      return EMPTY;
    }
  })();
  repoOptionsCache.set(key, { at: Date.now(), value });
  return value;
}
