import type { RepoOptions, ReportKind } from '../types';
import type { Annotator } from './annotator';
import { h, icon } from './dom';
import { icons } from './icons';
import { picker } from './picker';

export interface FormValues {
  title: string;
  description: string;
  includeContext: boolean;
  labels: string[];
  assignees: string[];
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
  /**
   * "Write it for me": drafts a title and description from the current notes and
   * screenshots. Omit to hide the button.
   */
  assist?: (values: FormValues) => Promise<{ title: string; description: string }>;
  /** Labels and assignees to offer; the pickers stay hidden until there's something to pick. */
  repoOptions?: Promise<RepoOptions>;
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

    // "Write it for me": fills title and description from the notes and screenshots.
    const assistBtn = h(
      'button',
      {
        type: 'button',
        class: 'ix-assist',
        title: 'Uses AI: sends your notes, screenshots and (if included) the technical context to the issue server, which asks Azure OpenAI for a draft. Review it before creating the issue.',
      },
      icon(icons.sparkle),
      h('span', {}, 'Write it for me'),
    );
    const assistNote = h('span', { class: 'ix-assist-note', hidden: true });
    const assistRow = opts.assist ? h('div', { class: 'ix-assist-row' }, assistBtn, assistNote) : null;
    let closed = false;
    let drafting = false;

    // Labels and assignees, as on GitHub. Hidden until the repo's options arrive.
    const labelPicker = picker({ id: 'ix-labels', label: 'Labels', placeholder: 'Add labels…', kind: 'label', selected: opts.initial?.labels });
    const assigneePicker = picker({ id: 'ix-assignees', label: 'Assignees', placeholder: 'Assign people…', kind: 'user', selected: opts.initial?.assignees });
    labelPicker.element.hidden = assigneePicker.element.hidden = true;
    void opts.repoOptions?.then((o) => {
      if (closed) return;
      labelPicker.setItems(o.labels.map((l) => ({ value: l.name, color: l.color, hint: l.description })));
      assigneePicker.setItems(o.assignees.map((a) => ({ value: a.login, avatar: a.avatarUrl })));
      labelPicker.element.hidden = o.labels.length === 0;
      assigneePicker.element.hidden = o.assignees.length === 0;
    });

    const [heading, sub] = HEADINGS[opts.kind];
    const form = h(
      'form',
      { class: 'ix-form', novalidate: true },
      h('h2', { id: 'ix-heading' }, heading),
      h('p', { class: 'ix-sub' }, shots.length > 1 ? 'Mark up the screenshots, then describe what went wrong.' : sub),
      h('div', { class: 'ix-field' }, h('label', { for: 'ix-title' }, 'Title'), title),
      h('div', { class: 'ix-field ix-grow' }, h('label', { for: 'ix-desc' }, 'Description'), description, assistRow),
      labelPicker.element,
      assigneePicker.element,
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
      labels: labelPicker.selected(),
      assignees: assigneePicker.selected(),
    });

    const setDrafting = (on: boolean) => {
      drafting = on;
      title.disabled = description.disabled = submitBtn.disabled = on;
      assistBtn.disabled = on;
      assistBtn.replaceChildren(
        on ? h('span', { class: 'ix-spin', 'aria-hidden': 'true' }) : icon(icons.sparkle),
        h('span', {}, on ? 'Writing…' : 'Write it for me'),
      );
      form.setAttribute('aria-busy', String(on));
    };
    const runAssist = async () => {
      if (!opts.assist || drafting) return;
      const before = values();
      if (!before.title && !before.description && !shots.length) {
        showError('Write a few words first, then let the assistant expand them.');
        description.focus();
        return;
      }
      error.hidden = true;
      assistNote.hidden = true;
      setDrafting(true);
      try {
        const draft = await opts.assist(before);
        if (closed) return;
        title.value = draft.title;
        description.value = draft.description;
        description.scrollTop = 0;
        const undo = h('button', { type: 'button', class: 'ix-link' }, 'Undo');
        undo.addEventListener('click', () => {
          title.value = before.title;
          description.value = before.description;
          assistNote.hidden = true;
        });
        assistNote.replaceChildren('Drafted by AI. Check it before creating the issue. ', undo);
        assistNote.hidden = false;
      } catch (err) {
        if (!closed) showError(err instanceof Error ? err.message : String(err));
      } finally {
        if (!closed) setDrafting(false);
      }
    };
    assistBtn.addEventListener('click', () => void runAssist());

    // Enter: submit when title and description are both filled in; otherwise let
    // the assistant fill them ("Write it for me"), or point at what's missing.
    const missing = () => (!title.value.trim() ? title : !description.value.trim() ? description : null);
    const proceed = () => {
      if (drafting) return;
      const gap = missing();
      if (!gap) {
        form.requestSubmit();
      } else if (opts.assist) {
        void runAssist();
      } else {
        showError(gap === title ? 'Please add a title.' : 'Please add a description.');
        gap.focus();
      }
    };
    title.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' && !e.isComposing) {
        e.preventDefault(); // no implicit form submission
        proceed();
      }
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
      strip.replaceChildren(...thumbs, ...(opts.canAdd ? [addMenu()] : []));
    };

    // "+" next to the thumbnails: choose Area or Element for another screenshot.
    let menuOpen: HTMLElement | null = null;
    const closeMenu = () => {
      if (!menuOpen) return;
      menuOpen.hidden = true;
      menuOpen.previousElementSibling?.setAttribute('aria-expanded', 'false');
      menuOpen = null;
    };
    function addMenu(): HTMLElement {
      const item = (mode: 'region' | 'element', svg: string, name: string, hint: string) =>
        h(
          'button',
          { type: 'button', role: 'menuitem', class: 'ix-add-item', onclick: () => close({ action: 'add', values: values(), mode }) },
          icon(svg),
          h('span', {}, h('b', {}, name), h('small', {}, hint)),
        );
      const menu = h(
        'div',
        { class: 'ix-add-menu', role: 'menu', hidden: true },
        item('region', icons.region, 'Area', 'Drag to select part of the page'),
        item('element', icons.element, 'Element', 'Click an element on the page'),
      );
      const plus = h(
        'button',
        { type: 'button', class: 'ix-shot-plus', title: 'Add another screenshot', 'aria-label': 'Add another screenshot', 'aria-haspopup': 'menu', 'aria-expanded': 'false' },
        icon(icons.plus),
      );
      plus.addEventListener('click', () => {
        if (menuOpen === menu) return closeMenu();
        closeMenu();
        menu.hidden = false;
        plus.setAttribute('aria-expanded', 'true');
        menuOpen = menu;
        (menu.querySelector('button') as HTMLButtonElement).focus();
      });
      return h('div', { class: 'ix-add-wrap' }, plus, menu);
    }

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
      if (drafting && result.action === 'submit') return;
      if (result.action === 'cancelled' && dirty() && !window.confirm('Discard this report?')) return;
      closed = true;
      window.removeEventListener('keydown', onKey, true);
      retake?.remove();
      backdrop.remove();
      if (videoUrl) URL.revokeObjectURL(videoUrl);
      resolve(result);
    };

    const handleKey = (e: KeyboardEvent): boolean => {
      if (e.key === 'Escape') {
        if (menuOpen) closeMenu();
        else close({ action: 'cancelled' });
        return true;
      }
      if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
        proceed();
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
      if (menuOpen && !e.composedPath().some((n) => n instanceof HTMLElement && n.classList.contains('ix-add-wrap'))) closeMenu();
      if (e.target === backdrop) close({ action: 'cancelled' });
    });

    form.addEventListener('submit', (e) => {
      e.preventDefault();
      const gap = missing();
      if (gap) {
        const what = gap === title ? 'a title' : 'a description';
        showError(opts.assist ? `Add ${what}, or use "Write it for me" to draft it.` : `Please add ${what}.`);
        gap.focus();
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
