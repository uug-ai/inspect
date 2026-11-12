import type { Rect } from '../types';
import { cssSelector, describeElement } from '../selector';
import { h } from './dom';

export interface Selection {
  region: Rect;
  selector?: string;
}

const MIN_SIZE = 8;

function fullViewport(): Rect {
  return { x: 0, y: 0, width: window.innerWidth, height: window.innerHeight };
}

function place(el: HTMLElement, r: Rect): void {
  Object.assign(el.style, { left: `${r.x}px`, top: `${r.y}px`, width: `${r.width}px`, height: `${r.height}px` });
}

/** Position a label just above (or inside, if no room) the rect. */
function placeTag(tag: HTMLElement, r: Rect): void {
  const above = r.y - 28;
  tag.style.left = `${Math.max(4, Math.min(r.x, window.innerWidth - tag.offsetWidth - 4))}px`;
  tag.style.top = `${above >= 4 ? above : Math.min(r.y + r.height + 6, window.innerHeight - 28)}px`;
}

function hint(text: string, actions: HTMLElement[]): HTMLElement {
  return h('div', { class: 'ix-hint' }, h('span', {}, text), ...actions);
}

/**
 * Let the user drag a rectangle. Resolves `null` on Escape / Cancel.
 * With `allowFull`, Enter or the "Whole page" button selects the full viewport.
 */
export function selectRegion(container: HTMLElement, opts: { text: string; allowFull?: boolean }): Promise<Selection | null> {
  return new Promise((resolve) => {
    const overlay = h('div', { class: 'ix-select ix-dim' });
    const box = h('div', { class: 'ix-sel-box', hidden: true });
    const tag = h('div', { class: 'ix-tag', hidden: true });
    const cancelBtn = h('button', { type: 'button' }, 'Cancel');
    const fullBtn = opts.allowFull ? h('button', { type: 'button', class: 'ix-primary' }, 'Whole page') : null;
    const bar = hint(opts.text, [fullBtn, cancelBtn].filter(Boolean) as HTMLElement[]);
    overlay.append(box, tag, bar);
    container.append(overlay);

    let start: { x: number; y: number } | null = null;
    let current: Rect | null = null;

    const finish = (result: Selection | null) => {
      overlay.remove();
      window.removeEventListener('keydown', onKey, true);
      resolve(result);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        e.stopPropagation();
        finish(null);
      } else if (e.key === 'Enter' && opts.allowFull) {
        e.preventDefault();
        finish({ region: fullViewport() });
      }
    };
    window.addEventListener('keydown', onKey, true);
    cancelBtn.addEventListener('click', () => finish(null));
    fullBtn?.addEventListener('click', () => finish({ region: fullViewport() }));
    bar.addEventListener('pointerdown', (e) => e.stopPropagation());

    overlay.addEventListener('pointerdown', (e) => {
      if (e.button !== 0) return;
      overlay.setPointerCapture(e.pointerId);
      start = { x: e.clientX, y: e.clientY };
      current = null;
      bar.hidden = true;
    });
    overlay.addEventListener('pointermove', (e) => {
      if (!start) return;
      const x = Math.max(0, Math.min(start.x, e.clientX));
      const y = Math.max(0, Math.min(start.y, e.clientY));
      current = {
        x,
        y,
        width: Math.min(window.innerWidth, Math.max(start.x, e.clientX)) - x,
        height: Math.min(window.innerHeight, Math.max(start.y, e.clientY)) - y,
      };
      overlay.classList.remove('ix-dim');
      box.hidden = false;
      tag.hidden = false;
      place(box, current);
      tag.textContent = `${Math.round(current.width)} × ${Math.round(current.height)}`;
      placeTag(tag, current);
    });
    overlay.addEventListener('pointerup', () => {
      const r = current;
      start = null;
      if (r && r.width >= MIN_SIZE && r.height >= MIN_SIZE) {
        finish({ region: r });
        return;
      }
      // Too small (or a plain click): reset and let them try again.
      box.hidden = true;
      tag.hidden = true;
      bar.hidden = false;
      overlay.classList.add('ix-dim');
    });
  });
}

/**
 * Hover to highlight the element under the cursor, click to pick it.
 * ↑ / ↓ walk to the parent / back to the child.
 */
export function pickElement(container: HTMLElement, host: Element): Promise<Selection | null> {
  return new Promise((resolve) => {
    const overlay = h('div', { class: 'ix-select ix-picking' });
    const box = h('div', { class: 'ix-pick-box', hidden: true });
    const tag = h('div', { class: 'ix-tag', hidden: true });
    const cancelBtn = h('button', { type: 'button' }, 'Cancel');
    const bar = hint('Click an element to capture it', [
      h('span', {}, h('kbd', {}, '↑'), ' ', h('kbd', {}, '↓'), ' parent / child'),
      cancelBtn,
    ]);
    overlay.append(box, tag, bar);
    container.append(overlay);

    let target: Element | null = null;
    const trail: Element[] = [];

    const show = (el: Element | null) => {
      target = el;
      if (!el) {
        box.hidden = tag.hidden = true;
        return;
      }
      const r = el.getBoundingClientRect();
      const rect = { x: r.left, y: r.top, width: r.width, height: r.height };
      box.hidden = tag.hidden = false;
      place(box, rect);
      tag.textContent = `${describeElement(el)}  ${Math.round(r.width)}×${Math.round(r.height)}`;
      placeTag(tag, rect);
    };
    const elementAt = (x: number, y: number) =>
      document.elementsFromPoint(x, y).find((el) => el !== host && !host.contains(el) && el !== document.documentElement) ?? null;

    const finish = (result: Selection | null) => {
      overlay.remove();
      window.removeEventListener('keydown', onKey, true);
      resolve(result);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        e.stopPropagation();
        finish(null);
      } else if (e.key === 'ArrowUp' && target?.parentElement && target.parentElement !== document.documentElement) {
        e.preventDefault();
        trail.push(target);
        show(target.parentElement);
      } else if (e.key === 'ArrowDown' && trail.length) {
        e.preventDefault();
        show(trail.pop()!);
      } else if (e.key === 'Enter' && target) {
        e.preventDefault();
        choose();
      }
    };
    const choose = () => {
      if (!target) return;
      const r = target.getBoundingClientRect();
      const x = Math.max(0, r.left);
      const y = Math.max(0, r.top);
      const region = {
        x,
        y,
        width: Math.min(window.innerWidth, r.right) - x,
        height: Math.min(window.innerHeight, r.bottom) - y,
      };
      finish(region.width > 0 && region.height > 0 ? { region, selector: cssSelector(target) } : null);
    };
    window.addEventListener('keydown', onKey, true);
    cancelBtn.addEventListener('click', () => finish(null));
    bar.addEventListener('pointerdown', (e) => e.stopPropagation());
    bar.addEventListener('pointermove', (e) => e.stopPropagation());

    overlay.addEventListener('pointermove', (e) => {
      const el = elementAt(e.clientX, e.clientY);
      if (el !== target) {
        trail.length = 0;
        show(el);
      }
    });
    overlay.addEventListener('click', (e) => {
      if (bar.contains(e.target as Node)) return;
      choose();
    });
  });
}
