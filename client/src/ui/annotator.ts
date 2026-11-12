import { h, icon } from './dom';
import { icons } from './icons';

type Tool = 'arrow' | 'rect' | 'pen' | 'text' | 'blur';
interface Pt {
  x: number;
  y: number;
}
type Shape =
  | { tool: 'arrow' | 'rect'; color: string; width: number; a: Pt; b: Pt }
  | { tool: 'pen'; color: string; width: number; points: Pt[] }
  | { tool: 'text'; color: string; size: number; at: Pt; text: string }
  | { tool: 'blur'; block: number; a: Pt; b: Pt };

const MAX_EDGE = 2560;
const WEBP_QUALITY = 0.9;

function encode(canvas: HTMLCanvasElement, type: string, quality?: number): Promise<Blob> {
  return new Promise((resolve, reject) =>
    canvas.toBlob((b) => (b ? resolve(b) : reject(new Error('Could not encode the screenshot'))), type, quality),
  );
}

function downscale(canvas: HTMLCanvasElement, maxEdge: number): HTMLCanvasElement {
  const scale = maxEdge / Math.max(canvas.width, canvas.height);
  if (scale >= 1) return canvas;
  const out = document.createElement('canvas');
  out.width = Math.round(canvas.width * scale);
  out.height = Math.round(canvas.height * scale);
  const ctx = out.getContext('2d')!;
  ctx.imageSmoothingQuality = 'high';
  ctx.drawImage(canvas, 0, 0, out.width, out.height);
  return out;
}

const COLORS = ['#e5484d', '#f5a524', '#30a46c', '#3e63dd', '#18181b'];
const TOOLS: Array<{ tool: Tool; label: string; key: string }> = [
  { tool: 'arrow', label: 'Arrow', key: 'a' },
  { tool: 'rect', label: 'Rectangle', key: 'r' },
  { tool: 'pen', label: 'Draw', key: 'p' },
  { tool: 'text', label: 'Text', key: 't' },
  { tool: 'blur', label: 'Pixelate (hide sensitive info)', key: 'b' },
];

function norm(a: Pt, b: Pt) {
  return { x: Math.min(a.x, b.x), y: Math.min(a.y, b.y), w: Math.abs(a.x - b.x), h: Math.abs(a.y - b.y) };
}

/** Screenshot editor: draws annotations on top of a captured frame. */
export class Annotator {
  readonly toolbar: HTMLElement;
  readonly stage: HTMLElement;
  private readonly canvas: HTMLCanvasElement;
  private readonly ctx: CanvasRenderingContext2D;
  private shapes: Shape[] = [];
  private draft: Shape | null = null;
  private tool: Tool = 'arrow';
  private color = COLORS[0];
  private undoBtn: HTMLButtonElement;
  private toolButtons = new Map<Tool, HTMLButtonElement>();
  private swatches: HTMLButtonElement[] = [];
  private textInput: HTMLInputElement | null = null;

  constructor(private readonly base: HTMLCanvasElement) {
    this.canvas = h('canvas', { width: base.width, height: base.height, 'aria-label': 'Screenshot — draw to annotate' });
    this.ctx = this.canvas.getContext('2d')!;
    this.stage = h('div', { class: 'ix-canvas-wrap' }, this.canvas);

    for (const t of TOOLS) {
      const btn = h(
        'button',
        { type: 'button', title: `${t.label} (${t.key.toUpperCase()})`, 'aria-label': t.label, onclick: () => this.setTool(t.tool) },
        icon(icons[t.tool]),
      );
      this.toolButtons.set(t.tool, btn);
    }
    for (const c of COLORS) {
      const btn = h('button', {
        type: 'button',
        class: 'ix-swatch',
        title: c,
        'aria-label': `Colour ${c}`,
        style: `background:${c}`,
        onclick: () => this.setColor(c),
      });
      this.swatches.push(btn);
    }
    this.undoBtn = h('button', { type: 'button', title: 'Undo (Ctrl+Z)', 'aria-label': 'Undo', onclick: () => this.undo() }, icon(icons.undo));
    this.toolbar = h(
      'div',
      { class: 'ix-tools', role: 'toolbar', 'aria-label': 'Annotation tools' },
      ...this.toolButtons.values(),
      h('span', { class: 'ix-sep' }),
      ...this.swatches,
      h('span', { class: 'ix-sep' }),
      this.undoBtn,
      h('span', { class: 'ix-spacer' }),
    );

    this.bindPointer();
    this.setTool('arrow');
    this.setColor(COLORS[0]);
    this.render();
  }

  /** Keyboard shortcuts while the dialog is open. Returns true if handled. */
  handleKey(e: KeyboardEvent): boolean {
    if (this.textInput) return false;
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'z') {
      this.undo();
      return true;
    }
    if (e.metaKey || e.ctrlKey || e.altKey) return false;
    const t = TOOLS.find((x) => x.key === e.key.toLowerCase());
    if (t) {
      this.setTool(t.tool);
      return true;
    }
    return false;
  }

  /** Small preview (with annotations) for the screenshot strip. */
  thumbnail(maxEdge = 160): string {
    const scale = Math.min(1, maxEdge / Math.max(this.canvas.width, this.canvas.height));
    const out = document.createElement('canvas');
    out.width = Math.max(1, Math.round(this.canvas.width * scale));
    out.height = Math.max(1, Math.round(this.canvas.height * scale));
    out.getContext('2d')!.drawImage(this.canvas, 0, 0, out.width, out.height);
    return out.toDataURL('image/jpeg', 0.75);
  }

  /**
   * The annotated screenshot, ready to upload. WebP is several times smaller than
   * PNG for UI screenshots, which speeds up both the upload and the GitHub commit;
   * browsers that can't encode WebP fall back to PNG. Very large captures are scaled
   * down so their longest side is at most MAX_EDGE pixels.
   */
  async toFile(): Promise<File> {
    this.commitText();
    this.draft = null;
    this.render();
    const source = downscale(this.canvas, MAX_EDGE);
    let blob = await encode(source, 'image/webp', WEBP_QUALITY);
    if (blob.type !== 'image/webp') blob = await encode(source, 'image/png');
    const ext = blob.type === 'image/webp' ? 'webp' : 'png';
    return new File([blob], `screenshot.${ext}`, { type: blob.type });
  }

  private setTool(tool: Tool): void {
    this.commitText();
    this.tool = tool;
    this.toolButtons.forEach((b, t) => b.setAttribute('aria-pressed', String(t === tool)));
    this.canvas.style.cursor = tool === 'text' ? 'text' : 'crosshair';
  }

  private setColor(color: string): void {
    this.color = color;
    this.swatches.forEach((b) => b.setAttribute('aria-pressed', String(b.title === color)));
  }

  private undo(): void {
    this.shapes.pop();
    this.render();
  }

  /** Image pixels per CSS pixel, so strokes look the same size regardless of zoom. */
  private scale(): number {
    const r = this.canvas.getBoundingClientRect();
    return r.width ? this.canvas.width / r.width : 1;
  }

  private toImage(e: PointerEvent | MouseEvent): Pt {
    const r = this.canvas.getBoundingClientRect();
    return { x: ((e.clientX - r.left) / r.width) * this.canvas.width, y: ((e.clientY - r.top) / r.height) * this.canvas.height };
  }

  private bindPointer(): void {
    const c = this.canvas;
    c.addEventListener('pointerdown', (e) => {
      if (e.button !== 0) return;
      if (this.tool === 'text') {
        e.preventDefault();
        this.commitText();
        this.startText(e);
        return;
      }
      c.setPointerCapture(e.pointerId);
      const p = this.toImage(e);
      const s = this.scale();
      if (this.tool === 'pen') this.draft = { tool: 'pen', color: this.color, width: 3 * s, points: [p] };
      else if (this.tool === 'blur') this.draft = { tool: 'blur', block: Math.max(6, Math.round(10 * s)), a: p, b: p };
      else this.draft = { tool: this.tool, color: this.color, width: 3 * s, a: p, b: p };
    });
    c.addEventListener('pointermove', (e) => {
      if (!this.draft) return;
      const p = this.toImage(e);
      if (this.draft.tool === 'pen') this.draft.points.push(p);
      else if ('b' in this.draft) this.draft.b = p;
      this.render();
    });
    const end = () => {
      const d = this.draft;
      this.draft = null;
      if (!d) return;
      const tiny = 'b' in d && Math.hypot(d.b.x - d.a.x, d.b.y - d.a.y) < 4 * this.scale();
      if (!tiny && !(d.tool === 'pen' && d.points.length < 2)) this.shapes.push(d);
      this.render();
    };
    c.addEventListener('pointerup', end);
    c.addEventListener('pointercancel', end);
  }

  private startText(e: PointerEvent): void {
    const at = this.toImage(e);
    const wrap = this.stage.getBoundingClientRect();
    const s = this.scale();
    const size = Math.round(20 * s);
    const input = h('input', {
      class: 'ix-text-input',
      type: 'text',
      placeholder: 'Type, then Enter',
      style: `left:${e.clientX - wrap.left}px;top:${e.clientY - wrap.top - 14}px;color:${this.color}`,
    });
    const commit = () => {
      if (this.textInput !== input) return;
      this.textInput = null;
      const text = input.value.trim();
      input.remove();
      if (text) {
        this.shapes.push({ tool: 'text', color: this.color, size, at, text });
        this.render();
      }
    };
    input.addEventListener('keydown', (ev) => {
      ev.stopPropagation();
      if (ev.key === 'Enter') commit();
      if (ev.key === 'Escape') {
        input.value = '';
        commit();
      }
    });
    input.addEventListener('blur', commit);
    this.textInput = input;
    this.stage.append(input);
    // Focus synchronously (pointerdown was preventDefault'ed) so no keystroke is lost.
    input.focus();
  }

  private commitText(): void {
    this.textInput?.blur();
  }

  private render(): void {
    const ctx = this.ctx;
    ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
    ctx.drawImage(this.base, 0, 0);
    for (const s of this.draft ? [...this.shapes, this.draft] : this.shapes) this.draw(s);
    this.undoBtn.disabled = this.shapes.length === 0;
  }

  private draw(s: Shape): void {
    const ctx = this.ctx;
    ctx.save();
    ctx.lineCap = 'round';
    ctx.lineJoin = 'round';
    switch (s.tool) {
      case 'rect': {
        const r = norm(s.a, s.b);
        ctx.strokeStyle = s.color;
        ctx.lineWidth = s.width;
        ctx.strokeRect(r.x, r.y, r.w, r.h);
        break;
      }
      case 'arrow': {
        const angle = Math.atan2(s.b.y - s.a.y, s.b.x - s.a.x);
        const head = s.width * 5;
        ctx.strokeStyle = ctx.fillStyle = s.color;
        ctx.lineWidth = s.width;
        ctx.beginPath();
        ctx.moveTo(s.a.x, s.a.y);
        ctx.lineTo(s.b.x - Math.cos(angle) * head * 0.6, s.b.y - Math.sin(angle) * head * 0.6);
        ctx.stroke();
        ctx.beginPath();
        ctx.moveTo(s.b.x, s.b.y);
        ctx.lineTo(s.b.x - head * Math.cos(angle - Math.PI / 7), s.b.y - head * Math.sin(angle - Math.PI / 7));
        ctx.lineTo(s.b.x - head * Math.cos(angle + Math.PI / 7), s.b.y - head * Math.sin(angle + Math.PI / 7));
        ctx.closePath();
        ctx.fill();
        break;
      }
      case 'pen': {
        ctx.strokeStyle = s.color;
        ctx.lineWidth = s.width;
        ctx.beginPath();
        s.points.forEach((p, i) => (i ? ctx.lineTo(p.x, p.y) : ctx.moveTo(p.x, p.y)));
        ctx.stroke();
        break;
      }
      case 'text': {
        ctx.font = `600 ${s.size}px system-ui, -apple-system, sans-serif`;
        ctx.textBaseline = 'middle';
        ctx.lineWidth = s.size / 5;
        ctx.strokeStyle = 'rgba(255,255,255,.95)';
        ctx.strokeText(s.text, s.at.x, s.at.y);
        ctx.fillStyle = s.color;
        ctx.fillText(s.text, s.at.x, s.at.y);
        break;
      }
      case 'blur': {
        const r = norm(s.a, s.b);
        if (r.w < 1 || r.h < 1) break;
        // Pixelate from the original frame so no detail survives under the block.
        const tw = Math.max(1, Math.round(r.w / s.block));
        const th = Math.max(1, Math.round(r.h / s.block));
        const tmp = document.createElement('canvas');
        tmp.width = tw;
        tmp.height = th;
        tmp.getContext('2d')!.drawImage(this.base, r.x, r.y, r.w, r.h, 0, 0, tw, th);
        ctx.imageSmoothingEnabled = false;
        ctx.drawImage(tmp, 0, 0, tw, th, r.x, r.y, r.w, r.h);
        break;
      }
    }
    ctx.restore();
  }
}
