import type { ReportKind } from '../types';
import type { Annotator } from './annotator';
import { h, icon } from './dom';
import { icons } from './icons';

export interface FormValues {
  title: string;
  description: string;
  includeContext: boolean;
}

export interface DialogOptions {
  kind: ReportKind;
  annotator?: Annotator;
  video?: Blob;
  /** Rows shown in the "context" disclosure, so the user sees what gets sent. */
  context: Array<[string, string]>;
  canRetake: boolean;
  /** Prefill the form, e.g. when reopening after a failed send. */
  initial?: FormValues;
  /** Error to show when the dialog opens (the previous send failed). */
  error?: string;
}

/** The dialog closes as soon as the user submits; sending happens in the background. */
export type DialogResult = { action: 'submit'; values: FormValues } | { action: 'cancelled' } | { action: 'retake' };

const HEADINGS: Record<ReportKind, [string, string]> = {
  screenshot: ['Report an issue', 'Mark up the screenshot, then describe what went wrong.'],
  recording: ['Report an issue', 'Check the recording, then describe what went wrong.'],
  note: ['Report an issue', 'Describe what went wrong or what you expected.'],
};

export function openDialog(container: HTMLElement, opts: DialogOptions): Promise<DialogResult> {
  return new Promise((resolve) => {
    let videoUrl: string | null = null;

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
      h('p', { class: 'ix-sub' }, sub),
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

    let stage: HTMLElement | null = null;
    if (opts.annotator) {
      stage = h('div', { class: 'ix-stage' }, opts.annotator.toolbar, opts.annotator.stage);
    } else if (opts.video) {
      videoUrl = URL.createObjectURL(opts.video);
      const tools = h('div', { class: 'ix-tools' }, h('span', { class: 'ix-spacer' }));
      stage = h(
        'div',
        { class: 'ix-stage' },
        tools,
        h('div', { class: 'ix-canvas-wrap' }, h('video', { src: videoUrl, controls: true, autoplay: true, muted: true, playsinline: true })),
      );
    }
    // The annotator's toolbar is reused when the dialog reopens after a failed send,
    // so the Retake button is removed again on close.
    let retake: HTMLButtonElement | null = null;
    if (stage && opts.canRetake) {
      retake = h('button', { type: 'button', class: 'ix-text-btn', title: 'Capture again' }, icon(icons.retake), 'Retake');
      retake.addEventListener('click', () => close({ action: 'retake' }));
      stage.querySelector('.ix-tools')!.append(retake);
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

    const dirty = () => !!(title.value.trim() || description.value.trim());
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
      return !typing && !!opts.annotator?.handleKey(e);
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
      close({
        action: 'submit',
        values: { title: title.value.trim(), description: description.value.trim(), includeContext: includeContext.checked },
      });
    });

    function showError(message: string) {
      error.replaceChildren(icon(icons.alert), h('span', {}, message));
      error.hidden = false;
    }
  });
}
