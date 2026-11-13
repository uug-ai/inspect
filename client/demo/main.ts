import { createInspect, type AssistRequest, type Report } from '../src';

// Animated canvas so screenshots/recordings have something "live" in them.
const canvas = document.getElementById('cam') as HTMLCanvasElement;
const ctx = canvas.getContext('2d')!;
let t = 0;
(function frame() {
  t += 1;
  ctx.fillStyle = '#111';
  ctx.fillRect(0, 0, canvas.width, canvas.height);
  ctx.fillStyle = '#6d4aff';
  ctx.fillRect(40 + ((t * 3) % (canvas.width - 120)), 120, 80, 80);
  ctx.fillStyle = '#fff';
  ctx.font = '20px monospace';
  ctx.fillText(new Date().toISOString(), 16, 32);
  requestAnimationFrame(frame);
})();

document.getElementById('throw')!.addEventListener('click', () => {
  console.error('Failed to load chart data', new Error('HTTP 500 from /api/events'));
});

// ?endpoint=http://localhost:8080 talks to a real inspect server; otherwise mock submits.
// ?repo=owner/name asks for a specific repo; by default the server's INSPECT_DEFAULT_REPO is used.
const params = new URLSearchParams(location.search);
const endpoint = params.get('endpoint') ?? undefined;
const repo = params.get('repo') ?? undefined;
const last = document.getElementById('last')!;

async function mockSubmit(report: Report) {
  await new Promise((r) => setTimeout(r, (window as unknown as { __delay?: number }).__delay ?? 600));
  const files = report.attachments.map((f) => `${f.name} (${f.type}, ${Math.round(f.size / 1024)} KB)`);
  last.textContent = JSON.stringify({ payload: report.payload, attachments: files }, null, 2);
  (window as unknown as { __lastReport: Report }).__lastReport = report;
  if (report.payload.title.toLowerCase().includes('fail')) throw new Error('Mock server error: try another title.');
  return { number: 42, url: 'https://github.com/uug-ai/inspect/issues/42' };
}

// Mock "Write it for me" (the real one runs on the inspect server with Azure OpenAI).
async function mockAssist(req: AssistRequest) {
  await new Promise((r) => setTimeout(r, 1200));
  const notes = (req.description || req.title).trim();
  return {
    title: `Events card: ${notes.slice(0, 60) || 'chart fails to load'}`,
    description: [
      '## Summary',
      `${notes || 'The chart on the Events card fails to load.'} (${req.images.length} screenshot(s) attached.)`,
      '',
      '## Steps to reproduce',
      `1. Open ${req.context.url ?? 'the page'}.`,
      '2. Look at the Events today card.',
      '',
      '## Expected behaviour',
      'The chart renders with today\'s events.',
      '',
      '## Actual behaviour',
      'The card shows "Chart failed to load (NaN%)".',
      '',
      '## Acceptance criteria',
      '- [ ] The chart renders when the events API answers.',
      '- [ ] A failed request shows a retry message instead of NaN%.',
      '',
      '## Test cases',
      '1. Given events exist, when the dashboard loads, then the chart shows them.',
      '2. Given the events API returns 500, when the dashboard loads, then a retry message is shown.',
    ].join('\n'),
  };
}

const inspect = createInspect({
  endpoint,
  repo,
  onSubmit: endpoint ? undefined : mockSubmit,
  onAssist: endpoint ? undefined : mockAssist,
  // With ?endpoint the server's repo labels and assignees are used; otherwise these samples.
  onOptions: endpoint
    ? undefined
    : async () => ({
        labels: [
          { name: 'bug', color: 'd73a4a', description: "Something isn't working" },
          { name: 'frontend', color: '1d76db', description: 'Hub web app' },
          { name: 'backend', color: '0e8a16', description: 'API and workers' },
          { name: 'ux', color: 'c5def5', description: 'Design and usability' },
          { name: 'priority: high', color: 'b60205' },
        ],
        assignees: [{ login: 'cedricve' }, { login: 'kilianboute' }, { login: 'uug-ai-bot' }],
      }),
  getContext: () => ({ version: '1.2.3', organisation: 'demo' }),
});
document.getElementById('open')!.addEventListener('click', () => inspect.open());
(window as unknown as { inspect: typeof inspect }).inspect = inspect;
