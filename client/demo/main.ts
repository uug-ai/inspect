import { createInspect, type Report } from '../src';

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
  await new Promise((r) => setTimeout(r, 600));
  const files = report.attachments.map((f) => `${f.name} (${f.type}, ${Math.round(f.size / 1024)} KB)`);
  last.textContent = JSON.stringify({ payload: report.payload, attachments: files }, null, 2);
  (window as unknown as { __lastReport: Report }).__lastReport = report;
  if (report.payload.title.toLowerCase().includes('fail')) throw new Error('Mock server error: try another title.');
  return { number: 42, url: 'https://github.com/uug-ai/inspect/issues/42' };
}

const inspect = createInspect({
  endpoint,
  repo,
  onSubmit: endpoint ? undefined : mockSubmit,
  getContext: () => ({ version: '1.2.3', organisation: 'demo' }),
});
document.getElementById('open')!.addEventListener('click', () => inspect.open());
(window as unknown as { inspect: typeof inspect }).inspect = inspect;
