import type { ReportKind } from '../types';
import type { Annotator } from './annotator';
import { h, icon } from './dom';
import { icons } from './icons';

export interface FormValues {
  title: string;
  description: string;
  includeContext: boolean;
}

export type AddMode = 'region' | 'element';

export interface DialogOptions {
  kind: ReportKind;
  /** Screenshots in this report, each with its own annotations. */
  shots?: Annotator[];
  /** Which screenshot is shown in the editor. */
  active?: number;
  /** Show "+ Area" / "+ Element" to capture another screenshot. */
  canAdd?: boolean;
  video?: Blob;
  /** Rows shown in the "context" disclosure, so the user sees what gets sent. */
  context: Array<[string, string]>;
  canRetake: boolean;
  /** Prefill the form, e.g. when reopening after a failed send or a new capture. */
  initial?: FormValues;
  /** Error to show when the dialog opens (the previous send failed). */
  error?: string;
}

/**
 * The dialog closes as soon as the user submits; sending happens in the background.
 * "retake" and "add" carry the form values so the dialog can reopen unchanged.
 */
export type DialogResult =
  | { action: 'submit'; values: FormValues }
  | { action: 'cancelled' }
  | { action: 'retake'; values: FormValues; active: number }
  | { action: 'add'; values: FormValues; mode: AddMode };

const HEADINGS: Record<ReportKind, [string, string]> = {
  screenshot: ['Report an issue', 'Mark up the screenshot, then describe what went wrong.'],
  recording: ['Report an issue', 'Check the recording, then describe what went wrong.'],
  note: ['Report an issue', 'Describe what went wrong or what you expected.'],
};

export function openDialog(container: HTMLElement, opts: DialogOptions): Promise<DialogResult> {
  return new Promise((resolve) => {
    let videoUrl: string | null = null;
    const shots = opts.shots ?? [];
    let active = Math.min(opts.active ?? 0, Math.max(0, shots.length - 1));

    const title = h('input', { id: 'ix-title', type: 'text', required: true, maxlength: 256, placeholder: 'Short summary', autocomplete: 'off' });
    const description = h('textarea', {
      id: 'ix-desc',
      maxlength: 20000,
      placeholder: 'What happened? What did you expect? Steps to reproduce…',
    });
    const includeContext = h('input', { type: 'checkbox', checked: true, 'aria-label': 'Include technical context' });
    const error = h('div', { class: 'ix-error', role: 'alert', hidden: true });
    const submitBtn = h('button', { type: 'submit', class: 'ix-btn ix-primary' }, 'Create issue');
    const cancelBtn = h('button', { type: 'button', class: 'ix-btn' }, 'Cancel');

    const [heading, sub] = HEADINGS[opts.kind];
    const form = h(
      'form',
      { class: 'ix-form', novalidate: true },
      h('h2', { id: 'ix-heading' }, heading),
      h('p', { class: 'ix-sub' }, shots.length > 1 ? 'Mark up the screenshots, then describe what went wrong.' : sub),
      h('div', { class: 'ix-field' }, h('label', { for: 'ix-title' }, 'Title'), title),
      h('div', { class: 'ix-field' }, h('label', { for: 'ix-desc' }, 'Description'), description),
      h(
        'details',
        { class: 'ix-context' },
        h(
          'summary',
          {},
          includeContext,
          h('span', {}, 'Include technical context'),
          h('em', {}, `${opts.context.length} fields`),
        ),
        h('dl', {}, ...opts.context.flatMap(([k, v]) => [h('dt', {}, k), h('dd', {}, v)])),
      ),
      error,
      h('div', { class: 'ix-actions' }, cancelBtn, submitBtn),
    );
    // Clicking the checkbox shouldn't toggle the disclosure.
    includeContext.addEventListener('click', (e) => e.stopPropagation());
    if (opts.initial) {
      title.value = opts.initial.title;
      description.value = opts.initial.description;
      includeContext.checked = opts.initial.includeContext;
    }
    const values = (): FormValues => ({
      title: title.value.trim(),
      description: description.value.trim(),
      includeContext: includeContext.checked,
    });

    // Retake lives in the active screenshot's toolbar. Toolbars are reused when the
    // dialog reopens (new capture, failed send), so it's moved and removed explicitly.
    const retake = opts.canRetake
      ? h('button', { type: 'button', class: 'ix-text-btn', title: 'Capture this screenshot again' }, icon(icons.retake), 'Retake')
      : null;
    retake?.addEventListener('click', () => close({ action: 'retake', values: values(), active }));

    let stage: HTMLElement | null = null;
    let strip: HTMLElement | null = null;
    const mountShot = () => {
      const a = shots[active];
      stage!.replaceChildren(a.toolbar, a.stage, ...(strip ? [strip] : []));
      if (retake) a.toolbar.append(retake);
      renderStrip();
    };
    const renderStrip = () => {
      if (!strip) return;
      const thumbs = shots.map((a, i) => {
        const pick = h(
          'button',
          {
            type: 'button',
            class: 'ix-shot',
            'aria-label': `Screenshot ${i + 1}`,
            'aria-pressed': String(i === active),
            title: `Screenshot ${i + 1}`,
          },
          h('img', { src: a.thumbnail(), alt: '' }),
          h('span', { class: 'ix-shot-n' }, String(i + 1)),
        );
        pick.addEventListener('click', () => {
          if (i === active) return;
          active = i;
          mountShot();
        });
        const wrap = h('div', { class: 'ix-shot-wrap' }, pick);
        if (shots.length > 1) {
          const remove = h('button', { type: 'button', class: 'ix-shot-remove', 'aria-label': `Remove screenshot ${i + 1}`, title: 'Remove' }, icon(icons.close));
          remove.addEventListener('click', () => {
            shots.splice(i, 1);
            if (active >= shots.length) active = shots.length - 1;
            else if (active > i) active--;
            mountShot();
          });
          wrap.append(remove);
        }
        return wrap;
      });
      const adds = opts.canAdd
        ? [
            h('button', { type: 'button', class: 'ix-shot-add', title: 'Add a screenshot of another area', onclick: () => close({ action: 'add', values: values(), mode: 'region' }) }, icon(icons.region), 'Area'),
            h('button', { type: 'button', class: 'ix-shot-add', title: 'Add a screenshot of an element', onclick: () => close({ action: 'add', values: values(), mode: 'element' }) }, icon(icons.element), 'Element'),
          ]
        : [];
      strip.replaceChildren(...thumbs, ...(adds.length ? [h('span', { class: 'ix-shot-label' }, 'Add')] : []), ...adds);
    };

    if (shots.length) {
      stage = h('div', { class: 'ix-stage' });
      if (shots.length > 1 || opts.canAdd) strip = h('div', { class: 'ix-shots', role: 'group', 'aria-label': 'Screenshots' });
      mountShot();
    } else if (opts.video) {
      videoUrl = URL.createObjectURL(opts.video);
      const tools = h('div', { class: 'ix-tools' }, h('span', { class: 'ix-spacer' }), retake);
      stage = h(
        'div',
        { class: 'ix-stage' },
        tools,
        h('div', { class: 'ix-canvas-wrap' }, h('video', { src: videoUrl, controls: true, autoplay: true, muted: true, playsinline: true })),
      );
    }

    const dialog = h(
      'div',
      { class: `ix-dialog${stage ? '' : ' ix-note'}`, role: 'dialog', 'aria-modal': 'true', 'aria-labelledby': 'ix-heading' },
      stage,
      form,
    );
    const backdrop = h('div', { class: 'ix-backdrop' }, dialog);
    container.append(backdrop);
    if (opts.error) showError(opts.error);
    requestAnimationFrame(() => title.focus());

    const dirty = () => !!(title.value.trim() || description.value.trim() || shots.length > 1);
    const close = (result: DialogResult) => {
      if (result.action === 'cancelled' && dirty() && !window.confirm('Discard this report?')) return;
      window.removeEventListener('keydown', onKey, true);
      retake?.remove();
      backdrop.remove();
      if (videoUrl) URL.revokeObjectURL(videoUrl);
      resolve(result);
    };

    const handleKey = (e: KeyboardEvent): boolean => {
      if (e.key === 'Escape') {
        close({ action: 'cancelled' });
        return true;
      }
      if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
        form.requestSubmit();
        return true;
      }
      const typing = e.composedPath().some((n) => n instanceof HTMLInputElement || n instanceof HTMLTextAreaElement);
      return !typing && !!shots[active]?.handleKey(e);
    };
    // Keys inside the dialog: handle, then stop them reaching the host app's shortcuts.
    // (Bubble phase, so our own inputs — e.g. the annotation text box — see them first.)
    backdrop.addEventListener('keydown', (e) => {
      if (handleKey(e)) e.preventDefault();
      e.stopPropagation();
    });
    // Keys while focus is outside the dialog (e.g. on <body> after clicking the canvas).
    const onKey = (e: KeyboardEvent) => {
      if (e.composedPath().includes(backdrop)) return;
      if (handleKey(e)) {
        e.preventDefault();
        e.stopPropagation();
      }
    };
    window.addEventListener('keydown', onKey, true);
    cancelBtn.addEventListener('click', () => close({ action: 'cancelled' }));
    backdrop.addEventListener('pointerdown', (e) => {
      if (e.target === backdrop) close({ action: 'cancelled' });
    });

    form.addEventListener('submit', (e) => {
      e.preventDefault();
      if (!title.value.trim()) {
        showError('Please add a title.');
        title.focus();
        return;
      }
      close({ action: 'submit', values: values() });
    });

    function showError(message: string) {
      error.replaceChildren(icon(icons.alert), h('span', {}, message));
      error.hidden = false;
    }
  });
}
