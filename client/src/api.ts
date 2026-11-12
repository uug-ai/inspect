import type { InspectOptions, Report, SubmitResult } from './types';

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
