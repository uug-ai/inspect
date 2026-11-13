import { h, icon } from './dom';
import { icons } from './icons';

export interface PickerItem {
  /** Value sent to the server (label name or login). */
  value: string;
  /** Extra text shown under the value in the list (label description). */
  hint?: string;
  /** Label colour (hex without '#'). */
  color?: string;
  /** Avatar image URL (assignees). */
  avatar?: string;
}

export interface Picker {
  element: HTMLElement;
  selected(): string[];
  setItems(items: PickerItem[]): void;
}

const MAX_SHOWN = 50;

/** A GitHub-style multi-select: chips for the selection, a searchable list below. */
export function picker(opts: { id: string; label: string; placeholder: string; selected?: string[]; kind: 'label' | 'user' }): Picker {
  let items: PickerItem[] = [];
  let chosen = [...(opts.selected ?? [])];

  const input = h('input', {
    id: opts.id,
    type: 'text',
    class: 'ix-picker-input',
    placeholder: opts.placeholder,
    autocomplete: 'off',
    role: 'combobox',
    'aria-expanded': 'false',
    'aria-controls': `${opts.id}-list`,
  });
  const chips = h('div', { class: 'ix-chips' });
  const box = h('div', { class: 'ix-picker' }, chips, input);
  const list = h('div', { class: 'ix-picker-list', id: `${opts.id}-list`, role: 'listbox', hidden: true });
  const element = h('div', { class: 'ix-field' }, h('label', { for: opts.id }, opts.label), box, list);

  const find = (value: string) => items.find((i) => i.value.toLowerCase() === value.toLowerCase());
  const swatch = (item: PickerItem | undefined, value: string) =>
    opts.kind === 'label'
      ? h('span', { class: 'ix-dot', style: `background:#${/^[0-9a-f]{6}$/i.test(item?.color ?? '') ? item!.color : '8b8b96'}` })
      : item?.avatar
        ? h('img', { class: 'ix-avatar', src: item.avatar, alt: '', referrerpolicy: 'no-referrer' })
        : h('span', { class: 'ix-avatar ix-initials' }, value.slice(0, 2).toUpperCase());

  const renderChips = () => {
    chips.replaceChildren(
      ...chosen.map((value) => {
        const remove = h('button', { type: 'button', class: 'ix-chip-x', 'aria-label': `Remove ${value}` }, icon(icons.close));
        remove.addEventListener('click', () => {
          chosen = chosen.filter((v) => v !== value);
          renderChips();
          renderList();
        });
        return h('span', { class: 'ix-chip' }, swatch(find(value), value), h('span', {}, value), remove);
      }),
    );
  };

  const matches = () => {
    const q = input.value.trim().toLowerCase();
    return items.filter((i) => !q || i.value.toLowerCase().includes(q) || (i.hint ?? '').toLowerCase().includes(q));
  };

  const toggle = (value: string) => {
    chosen = chosen.some((v) => v.toLowerCase() === value.toLowerCase())
      ? chosen.filter((v) => v.toLowerCase() !== value.toLowerCase())
      : [...chosen, value];
    input.value = '';
    renderChips();
    renderList();
  };

  const renderList = () => {
    const found = matches();
    if (!found.length) {
      list.replaceChildren(h('div', { class: 'ix-picker-empty' }, items.length ? 'No matches' : 'Nothing to choose from'));
      return;
    }
    list.replaceChildren(
      ...found.slice(0, MAX_SHOWN).map((item) => {
        const on = chosen.some((v) => v.toLowerCase() === item.value.toLowerCase());
        const option = h(
          'div',
          { class: 'ix-option', role: 'option', 'aria-selected': String(on) },
          h('span', { class: 'ix-check' }, on ? icon(icons.check) : null),
          swatch(item, item.value),
          h('span', { class: 'ix-option-text' }, h('span', {}, item.value), item.hint ? h('small', {}, item.hint) : null),
        );
        // pointerdown keeps focus in the input, so the list doesn't close before the click.
        option.addEventListener('pointerdown', (e) => {
          e.preventDefault();
          toggle(item.value);
        });
        return option;
      }),
    );
  };

  const open = (show: boolean) => {
    list.hidden = !show;
    input.setAttribute('aria-expanded', String(show));
    if (show) renderList();
  };
  input.addEventListener('focus', () => open(true));
  input.addEventListener('blur', () => open(false));
  input.addEventListener('input', () => open(true));
  input.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && !list.hidden) {
      e.preventDefault();
      e.stopPropagation(); // close the list, not the dialog
      open(false);
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const first = matches()[0];
      if (first) toggle(first.value);
    } else if (e.key === 'Backspace' && !input.value && chosen.length) {
      chosen = chosen.slice(0, -1);
      renderChips();
      renderList();
    }
  });
  box.addEventListener('pointerdown', (e) => {
    if (e.target === box || e.target === chips) {
      e.preventDefault();
      input.focus();
    }
  });

  renderChips();
  return {
    element,
    selected: () => [...chosen],
    setItems(next) {
      items = next;
      // Keep only picks that exist in the repository.
      chosen = chosen.filter((v) => find(v)).map((v) => find(v)!.value);
      renderChips();
      if (!list.hidden) renderList();
    },
  };
}
