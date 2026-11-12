const MAX_DEPTH = 6;

function escape(value: string): string {
  return typeof CSS !== 'undefined' && CSS.escape ? CSS.escape(value) : value.replace(/[^a-zA-Z0-9_-]/g, '\\$&');
}

/** Classes that look generated (Angular `_ngcontent-…`, CSS modules hashes, …) make poor selectors. */
function usefulClasses(el: Element): string[] {
  return Array.from(el.classList)
    .filter((c) => !/^(ng-|_|cdk-|mat-mdc-)|[0-9a-f]{5,}/i.test(c))
    .slice(0, 2);
}

function isUnique(selector: string): boolean {
  try {
    return document.querySelectorAll(selector).length === 1;
  } catch {
    return false;
  }
}

/** Build a short, human-readable CSS selector that uniquely identifies `el` if possible. */
export function cssSelector(el: Element): string {
  const parts: string[] = [];
  let node: Element | null = el;

  for (let depth = 0; node && node !== document.documentElement && depth < MAX_DEPTH; depth++) {
    for (const attr of ['data-testid', 'data-test', 'data-cy']) {
      const v = node.getAttribute(attr);
      if (v) {
        parts.unshift(`[${attr}="${v.replace(/"/g, '\\"')}"]`);
        return parts.join(' > ');
      }
    }
    if (node.id && !/\d{3,}/.test(node.id)) {
      parts.unshift(`#${escape(node.id)}`);
      return parts.join(' > ');
    }

    let part = node.tagName.toLowerCase();
    const classes = usefulClasses(node);
    if (classes.length) part += classes.map((c) => `.${escape(c)}`).join('');

    const parent: Element | null = node.parentElement;
    if (parent) {
      const same = Array.from(parent.children).filter((c) => c.tagName === node!.tagName);
      if (same.length > 1) part += `:nth-of-type(${same.indexOf(node) + 1})`;
    }
    parts.unshift(part);

    const candidate = parts.join(' > ');
    if (isUnique(candidate)) return candidate;
    node = parent;
  }
  return parts.join(' > ');
}

/** Short label for the hover tooltip, e.g. `button.primary  120×32`. */
export function describeElement(el: Element): string {
  let label = el.tagName.toLowerCase();
  if (el.id) label += `#${el.id}`;
  const classes = usefulClasses(el);
  if (classes.length) label += classes.map((c) => `.${c}`).join('');
  return label;
}
