import { requestAssist, serverCapabilities, submitReport } from './api';
import { CaptureUnsupportedError, Recording, ScreenSession } from './capture';
import { ConsoleBuffer } from './console';
import type { Inspect, InspectOptions, Rect, ReportContext, ReportKind } from './types';
import { Annotator } from './ui/annotator';
import { openDialog, type FormValues } from './ui/dialog';
import { h, icon } from './ui/dom';
import { icons } from './ui/icons';
import { busy, recordingControls, toast, toolbar, type ToolbarMode } from './ui/overlays';
import { pickElement, selectRegion, type Selection } from './ui/select';
import { styles } from './ui/styles';

interface Shortcut {
  code: string;
  alt: boolean;
  shift: boolean;
  ctrl: boolean;
  meta: boolean;
  label: string;
}

/** The server accepts up to five attachments per report. */
const MAX_SHOTS = 5;

interface Shot {
  mode: 'region' | 'element';
  selection: Selection;
  annotator: Annotator;
}

const isMac = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform);

function parseShortcut(spec: string): Shortcut | null {
  const parts = spec.toLowerCase().split('+').map((p) => p.trim());
  const key = parts.pop();
  if (!key) return null;
  const has = (m: string) => parts.includes(m);
  const s = { alt: has('alt') || has('option'), shift: has('shift'), ctrl: has('ctrl') || has('control'), meta: has('meta') || has('cmd') };
  const code = /^[a-z]$/.test(key) ? `Key${key.toUpperCase()}` : /^\d$/.test(key) ? `Digit${key}` : key;
  const label = isMac
    ? `${s.ctrl ? '⌃' : ''}${s.alt ? '⌥' : ''}${s.shift ? '⇧' : ''}${s.meta ? '⌘' : ''}${key.toUpperCase()}`
    : [s.ctrl && 'Ctrl', s.alt && 'Alt', s.shift && 'Shift', s.meta && 'Meta', key.toUpperCase()].filter(Boolean).join('+');
  return { ...s, code, label };
}

function shortUserAgent(ua: string): string {
  const browser = /(Edg|OPR|Firefox|Chrome|Version)\/([\d]+)/.exec(ua);
  const name = browser ? ({ Edg: 'Edge', OPR: 'Opera', Version: 'Safari' } as Record<string, string>)[browser[1]] ?? browser[1] : 'Browser';
  const os = /Windows|Mac OS X|Android|iPhone|iPad|Linux/.exec(ua)?.[0]?.replace('Mac OS X', 'macOS') ?? '';
  return `${name}${browser ? ` ${browser[2]}` : ''}${os ? ` on ${os}` : ''}`;
}

export class Controller implements Inspect {
  private readonly host: HTMLElement;
  private readonly layer: HTMLElement;
  private readonly launcher: HTMLButtonElement | null = null;
  private readonly session: ScreenSession;
  private readonly consoleBuffer: ConsoleBuffer | null = null;
  private readonly shortcut: Shortcut | null;
  private bar: HTMLElement | null = null;
  private busyFlow = false;
  private recording: { rec: Recording; stop: () => void } | null = null;

  constructor(private readonly options: InspectOptions) {
    this.host = h('div', { 'data-uug-inspect': '' });
    Object.assign(this.host.style, {
      position: 'fixed',
      inset: '0',
      pointerEvents: 'none',
      zIndex: String(options.zIndex ?? 2147483000),
    });
    const root = this.host.attachShadow({ mode: 'open' });
    const style = h('style', {}, styles);
    this.layer = h('div', {});
    if (options.accentColor) this.host.style.setProperty('--ix-accent', options.accentColor);
    root.append(style, this.layer);

    this.session = new ScreenSession(() => this.recording?.stop());
    this.shortcut = options.shortcut === false ? null : parseShortcut(options.shortcut ?? 'alt+shift+i');

    if (options.launcher !== false) {
      const position = (typeof options.launcher === 'object' && options.launcher.position) || 'bottom-right';
      this.launcher = h(
        'button',
        {
          type: 'button',
          class: 'ix-launcher',
          'data-pos': position,
          title: `Report an issue${this.shortcut ? ` (${this.shortcut.label})` : ''}`,
          'aria-label': 'Report an issue',
          onclick: () => this.open(),
        },
        icon(icons.bug),
      );
      this.layer.append(this.launcher);
    }

    if (options.captureConsole !== false) {
      this.consoleBuffer = new ConsoleBuffer();
      this.consoleBuffer.install();
    }
    window.addEventListener('keydown', this.onGlobalKey, true);
    document.body.append(this.host);
  }

  open(): void {
    if (this.busyFlow || this.bar) return;
    void this.assistAvailable(); // warm the capabilities check before the dialog opens
    this.setLauncherVisible(false);
    this.bar = toolbar((mode) => this.start(mode), () => this.close(), this.shortcut?.label);
    this.layer.append(this.bar);
    (this.bar.querySelector('button') as HTMLButtonElement | null)?.focus();
  }

  close(): void {
    this.bar?.remove();
    this.bar = null;
    this.session.release();
    if (!this.busyFlow) this.setLauncherVisible(true);
  }

  destroy(): void {
    this.recording?.rec.stop();
    this.close();
    window.removeEventListener('keydown', this.onGlobalKey, true);
    this.consoleBuffer?.uninstall();
    this.host.remove();
  }

  start(mode: ToolbarMode): void {
    if (this.busyFlow) return;
    this.bar?.remove();
    this.bar = null;
    this.layer.querySelectorAll('.ix-toast').forEach((t) => t.remove());
    this.setLauncherVisible(false);
    this.busyFlow = true;
    this.run(mode)
      .catch((err) => {
        console.warn('[inspect]', err);
        toast(this.layer, { message: err instanceof Error ? err.message : String(err), error: true });
      })
      .finally(() => {
        this.busyFlow = false;
        this.session.release();
        this.setLauncherVisible(true);
      });
  }

  private onGlobalKey = (e: KeyboardEvent) => {
    const s = this.shortcut;
    if (s && e.code === s.code && e.altKey === s.alt && e.shiftKey === s.shift && e.ctrlKey === s.ctrl && e.metaKey === s.meta) {
      e.preventDefault();
      if (this.bar) this.close();
      else this.open();
      return;
    }
    if (e.key === 'Escape' && this.bar) {
      e.preventDefault();
      this.close();
    }
  };

  private setLauncherVisible(visible: boolean): void {
    if (this.launcher) this.launcher.hidden = !visible;
  }

  /** One full report flow. Returns when the report was sent or abandoned. */
  private async run(mode: ToolbarMode): Promise<void> {
    if (mode === 'note') {
      await this.report('note', {});
      return;
    }
    if (mode === 'region' || mode === 'element') {
      const shot = await this.captureShot(mode);
      if (shot) await this.report('screenshot', { shots: [shot] });
      return;
    }

    // Recording: one video per report; Retake records again.
    if (!(await this.ensureCapture())) return;
    for (;;) {
      const selection = await selectRegion(this.layer, { text: 'Drag to select the area to record', allowFull: true });
      if (!selection) return;
      const video = await this.recordRegion(selection.region);
      if (!video) return;
      const result = await this.report('recording', { selection, video });
      if (result !== 'retake') return;
      if (!this.session.active && !(await this.ensureCapture())) return;
    }
  }

  /** Select an area or element and grab it. Null when the person cancels. */
  private async captureShot(mode: 'region' | 'element'): Promise<Shot | null> {
    if (!(await this.ensureCapture())) return null;
    const selection =
      mode === 'element'
        ? await pickElement(this.layer, this.host)
        : await selectRegion(this.layer, { text: 'Drag to select an area', allowFull: true });
    if (!selection) return null;
    const frame = await this.grab(selection.region);
    return { mode, selection, annotator: new Annotator(frame) };
  }

  private async ensureCapture(): Promise<boolean> {
    if (this.session.active) return true;
    const notice = busy(isMac ? 'Choose “This tab” in the browser prompt to continue…' : 'Choose this tab in the browser prompt to continue…');
    this.layer.append(notice);
    try {
      await this.session.ensure();
      // The "sharing this tab" infobar can resize the viewport; let layout settle.
      await new Promise((r) => setTimeout(r, 300));
      return true;
    } catch (err) {
      if (err instanceof CaptureUnsupportedError) throw err;
      const denied = err instanceof DOMException && (err.name === 'NotAllowedError' || err.name === 'AbortError');
      if (!denied) throw err;
      toast(this.layer, { message: 'Screen capture was cancelled. You can still send a note without a capture.', error: true });
      return false;
    } finally {
      notice.remove();
    }
  }

  /** Hide our own UI, wait for a clean frame, and grab it. */
  private async grab(region: Rect): Promise<HTMLCanvasElement> {
    this.host.style.visibility = 'hidden';
    try {
      await this.session.settle();
      return this.session.grab(region);
    } finally {
      this.host.style.visibility = '';
    }
  }

  private recordRegion(region: Rect): Promise<Blob | null> {
    return new Promise((resolve) => {
      const rec = this.session.record(region);
      let discarded = false;
      const finish = async () => {
        if (!this.recording) return;
        this.recording = null;
        controls.remove();
        const blob = await rec.stop();
        resolve(discarded || blob.size === 0 ? null : blob);
      };
      const controls = recordingControls(this.layer, region, this.options.maxRecordingSeconds ?? 120, {
        onStop: finish,
        onCancel: () => {
          discarded = true;
          finish();
        },
      });
      this.recording = { rec, stop: finish };
    });
  }

  private async buildContext(selection?: Selection): Promise<ReportContext> {
    let app: Record<string, string> | undefined;
    if (this.options.getContext) {
      try {
        const raw = await this.options.getContext();
        app = Object.fromEntries(
          Object.entries(raw ?? {})
            .filter(([, v]) => v !== undefined && v !== null)
            .map(([k, v]) => [k, typeof v === 'string' ? v : JSON.stringify(v)]),
        );
      } catch (err) {
        console.warn('[inspect] getContext failed', err);
      }
    }
    const round = (r: Rect) => ({ x: Math.round(r.x), y: Math.round(r.y), width: Math.round(r.width), height: Math.round(r.height) });
    const consoleEntries = this.consoleBuffer?.snapshot();
    return {
      timestamp: new Date().toISOString(),
      url: location.href,
      title: document.title,
      userAgent: navigator.userAgent,
      viewport: { width: window.innerWidth, height: window.innerHeight, dpr: window.devicePixelRatio || 1 },
      selector: selection?.selector,
      region: selection ? round(selection.region) : undefined,
      console: consoleEntries?.length ? consoleEntries : undefined,
      app: app && Object.keys(app).length ? app : undefined,
    };
  }

  private contextRows(c: ReportContext): Array<[string, string]> {
    const rows: Array<[string, string]> = [
      ['Page', c.url ?? ''],
      ['Browser', shortUserAgent(c.userAgent ?? '')],
      ['Viewport', c.viewport ? `${c.viewport.width}×${c.viewport.height} @${c.viewport.dpr}x` : ''],
    ];
    if (c.selector) rows.push(['Element', c.selector]);
    const errors = c.console?.filter((e) => e.level === 'error').length ?? 0;
    const warnings = (c.console?.length ?? 0) - errors;
    if (c.console) rows.push(['Console', `${errors} error${errors === 1 ? '' : 's'}, ${warnings} warning${warnings === 1 ? '' : 's'}`]);
    for (const [k, v] of Object.entries(c.app ?? {})) rows.push([k, v]);
    return rows;
  }

  /** Whether to offer "Write it for me": a custom onAssist, or the server has it on. */
  private assistAvailable(): Promise<boolean> {
    const custom = this.options.onAssist;
    if (custom === false) return Promise.resolve(false);
    if (typeof custom === 'function') return Promise.resolve(true);
    if (this.options.onSubmit && !this.options.endpoint) return Promise.resolve(false);
    return serverCapabilities(this.options).then((c) => c.assist);
  }

  private async report(
    kind: ReportKind,
    capture: { shots?: Shot[]; selection?: Selection; video?: Blob },
  ): Promise<'submitted' | 'cancelled' | 'retake'> {
    const shots = capture.shots ?? [];
    // Context (region, selector) describes the first capture.
    const context = await this.buildContext(shots[0]?.selection ?? capture.selection);
    let active = 0;
    let values: FormValues | undefined;
    let error: string | undefined;
    const canAssist = await this.assistAvailable();
    const assist = canAssist
      ? async (v: FormValues) =>
          requestAssist(this.options, {
            title: v.title,
            description: v.description,
            kind,
            context: v.includeContext ? context : { timestamp: context.timestamp },
            images: await Promise.all(shots.map((s) => s.annotator.toFile())),
          })
      : undefined;

    for (;;) {
      const annotators = shots.map((s) => s.annotator);
      const result = await openDialog(this.layer, {
        kind,
        shots: annotators,
        active,
        canAdd: kind === 'screenshot' && shots.length < MAX_SHOTS,
        video: capture.video,
        context: this.contextRows(context),
        canRetake: kind !== 'note',
        initial: values,
        error,
        assist,
      });
      // The dialog may have removed screenshots from `annotators`; keep `shots` in step.
      for (let i = shots.length - 1; i >= 0; i--) if (!annotators.includes(shots[i].annotator)) shots.splice(i, 1);
      error = undefined;

      if (result.action === 'cancelled') return 'cancelled';
      if (result.action === 'retake' && kind !== 'screenshot') return 'retake';
      if (result.action === 'retake' || result.action === 'add') {
        values = result.values;
        const replacing = result.action === 'retake';
        const index = replacing ? result.active : shots.length;
        const shot = await this.captureShot(replacing ? shots[index].mode : result.mode);
        if (shot) shots.splice(index, replacing ? 1 : 0, shot);
        active = shot ? index : Math.min(replacing ? result.active : active, shots.length - 1);
        continue;
      }
      values = result.values;

      // The dialog is closed: don't keep the person waiting for GitHub. Stop sharing
      // the tab right away and report progress in a toast instead.
      this.session.release();
      const sending = toast(this.layer, { message: 'Sending report…', busy: true });
      try {
        const attachments: File[] = [];
        for (const shot of shots) attachments.push(await shot.annotator.toFile());
        if (capture.video) {
          const ext = capture.video.type.includes('mp4') ? 'mp4' : 'webm';
          attachments.push(new File([capture.video], `recording.${ext}`, { type: capture.video.type }));
        }
        const sent = await whileSending(() =>
          submitReport(this.options, {
            payload: {
              title: values!.title,
              description: values!.description,
              kind,
              repo: this.options.repo,
              labels: this.options.labels,
              context: values!.includeContext ? context : { timestamp: context.timestamp },
            },
            attachments,
          }),
        );
        sending.update({ message: sent.number ? `Issue #${sent.number} created` : 'Issue created', href: sent.url });
        return 'submitted';
      } catch (err) {
        // Reopen the report with everything the person entered, plus the error.
        sending.remove();
        error = err instanceof Error ? err.message : String(err);
      }
    }
  }
}

/** Warn before the page is closed while a report is still being sent. */
async function whileSending<T>(send: () => Promise<T>): Promise<T> {
  const warn = (e: BeforeUnloadEvent) => {
    e.preventDefault();
    e.returnValue = '';
  };
  window.addEventListener('beforeunload', warn);
  try {
    return await send();
  } finally {
    window.removeEventListener('beforeunload', warn);
  }
}
