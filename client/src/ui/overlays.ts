import type { Rect } from '../types';
import { h, icon } from './dom';
import { icons } from './icons';

export type ToolbarMode = 'region' | 'element' | 'record' | 'note';

export function toolbar(onPick: (mode: ToolbarMode) => void, onClose: () => void, shortcut?: string): HTMLElement {
  const btn = (mode: ToolbarMode, label: string, svg: string, title: string) =>
    h('button', { type: 'button', 'data-mode': mode, title, onclick: () => onPick(mode) }, icon(svg), h('span', { class: 'ix-label' }, label));
  return h(
    'div',
    { class: 'ix-toolbar', role: 'toolbar', 'aria-label': 'Report an issue' },
    btn('region', 'Area', icons.region, 'Screenshot an area'),
    btn('element', 'Element', icons.element, 'Screenshot an element'),
    btn('record', 'Record', icons.record, 'Record a video'),
    btn('note', 'Note', icons.note, 'Report without a capture'),
    h('span', { class: 'ix-sep' }),
    shortcut ? h('kbd', { title: 'Shortcut' }, shortcut) : null,
    h('button', { type: 'button', class: 'ix-close', title: 'Close (Esc)', 'aria-label': 'Close', onclick: onClose }, icon(icons.close)),
  );
}

export function busy(text: string): HTMLElement {
  return h('div', { class: 'ix-busy', role: 'status' }, text);
}

/**
 * Dashed frame around the recorded area plus a floating stop button.
 * The frame doesn't catch clicks, so the page stays usable while recording.
 */
export function recordingControls(
  container: HTMLElement,
  region: Rect,
  maxSeconds: number,
  handlers: { onStop: () => void; onCancel: () => void },
): { remove: () => void } {
  const frame = h('div', { class: 'ix-rec-frame' });
  Object.assign(frame.style, { left: `${region.x}px`, top: `${region.y}px`, width: `${region.width}px`, height: `${region.height}px` });
  const time = h('span', {}, '0:00');
  const stop = h('button', { type: 'button', title: 'Stop recording' }, icon(icons.stop), 'Stop');
  const cancel = h('button', { type: 'button', class: 'ix-ghost', title: 'Discard recording', 'aria-label': 'Discard recording' }, icon(icons.close));
  const bar = h('div', { class: 'ix-rec-bar', role: 'status' }, h('span', { class: 'ix-rec-dot' }), time, stop, cancel);
  container.append(frame, bar);

  // Keep the controls outside the recorded area when there's room.
  const barH = 40;
  const below = region.y + region.height + 12;
  const above = region.y - barH - 12;
  const top = below + barH <= window.innerHeight ? below : above >= 0 ? above : window.innerHeight - barH - 16;
  bar.style.top = `${top}px`;
  requestAnimationFrame(() => {
    const left = region.x + region.width / 2 - bar.offsetWidth / 2;
    bar.style.left = `${Math.max(8, Math.min(left, window.innerWidth - bar.offsetWidth - 8))}px`;
  });

  const started = Date.now();
  const timer = setInterval(() => {
    const s = Math.floor((Date.now() - started) / 1000);
    time.textContent = `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
    if (s >= maxSeconds) handlers.onStop();
  }, 250);

  const onKey = (e: KeyboardEvent) => {
    if (e.key === 'Escape') {
      e.preventDefault();
      handlers.onStop();
    }
  };
  window.addEventListener('keydown', onKey, true);
  stop.addEventListener('click', handlers.onStop);
  cancel.addEventListener('click', handlers.onCancel);

  return {
    remove() {
      clearInterval(timer);
      window.removeEventListener('keydown', onKey, true);
      frame.remove();
      bar.remove();
    },
  };
}

export interface ToastOptions {
  message: string;
  href?: string;
  error?: boolean;
  /** Show a spinner and stay until updated or removed (no auto-dismiss, no close button). */
  busy?: boolean;
}

export interface Toast {
  update(opts: ToastOptions): void;
  remove(): void;
}

export function toast(container: HTMLElement, initial: ToastOptions): Toast {
  const el = h('div', { role: 'status', 'aria-live': 'polite' });
  let timer: ReturnType<typeof setTimeout> | undefined;
  const remove = () => {
    clearTimeout(timer);
    el.remove();
  };
  const render = (opts: ToastOptions) => {
    clearTimeout(timer);
    el.className = `ix-toast${opts.error ? ' ix-toast-error' : ''}${opts.busy ? ' ix-toast-busy' : ''}`;
    const close = opts.busy ? null : h('button', { type: 'button', 'aria-label': 'Dismiss', onclick: remove }, icon(icons.close));
    const parts: Node[] = [
      opts.busy ? h('span', { class: 'ix-spin', 'aria-hidden': 'true' }) : icon(opts.error ? icons.alert : icons.check),
      h('span', {}, opts.message),
    ];
    if (opts.href) parts.push(h('a', { href: opts.href, target: '_blank', rel: 'noopener noreferrer' }, 'Open', icon(icons.external)));
    if (close) parts.push(close);
    el.replaceChildren(...parts);
    if (!opts.busy) timer = setTimeout(remove, opts.href ? 12000 : 6000);
  };
  container.querySelectorAll('.ix-toast').forEach((t) => t.remove());
  render(initial);
  container.append(el);
  return { update: render, remove };
}
