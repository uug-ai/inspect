const svg = (body: string) =>
  `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">${body}</svg>`;

export const icons = {
  bug: svg(
    '<path d="M8 2l1.9 1.9"/><path d="M14.1 3.9L16 2"/><path d="M9 7.1V6a3 3 0 0 1 6 0v1.1"/><path d="M12 20c-3.3 0-6-2.7-6-6v-3a4 4 0 0 1 4-4h4a4 4 0 0 1 4 4v3c0 3.3-2.7 6-6 6z"/><path d="M12 20v-9"/><path d="M6.5 13H3"/><path d="M21 13h-3.5"/><path d="M6 9.5L3.5 8"/><path d="M18 9.5L20.5 8"/><path d="M6 17l-2.5 1.5"/><path d="M18 17l2.5 1.5"/>',
  ),
  region: svg(
    '<path d="M3 7V5a2 2 0 0 1 2-2h2"/><path d="M17 3h2a2 2 0 0 1 2 2v2"/><path d="M21 17v2a2 2 0 0 1-2 2h-2"/><path d="M7 21H5a2 2 0 0 1-2-2v-2"/><rect x="7" y="7" width="10" height="10" rx="1"/>',
  ),
  element: svg('<path d="M4 4l7 17 2.5-7.5L21 11z"/><path d="M14 14l6 6"/>'),
  record: svg('<circle cx="12" cy="12" r="9"/><circle cx="12" cy="12" r="4" fill="currentColor"/>'),
  note: svg('<path d="M14 3H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"/><path d="M14 3v6h6"/><path d="M8 13h8"/><path d="M8 17h5"/>'),
  close: svg('<path d="M18 6L6 18"/><path d="M6 6l12 12"/>'),
  stop: svg('<rect x="6" y="6" width="12" height="12" rx="2" fill="currentColor"/>'),
  arrow: svg('<path d="M5 19L19 5"/><path d="M9 5h10v10"/>'),
  rect: svg('<rect x="4" y="5" width="16" height="14" rx="1.5"/>'),
  pen: svg('<path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4z"/>'),
  text: svg('<path d="M4 7V5h16v2"/><path d="M12 5v14"/><path d="M9 19h6"/>'),
  blur: svg(
    '<rect x="3" y="3" width="6" height="6" fill="currentColor" opacity=".9"/><rect x="9" y="9" width="6" height="6" fill="currentColor" opacity=".5"/><rect x="15" y="3" width="6" height="6" fill="currentColor" opacity=".3"/><rect x="3" y="15" width="6" height="6" fill="currentColor" opacity=".3"/><rect x="15" y="15" width="6" height="6" fill="currentColor" opacity=".9"/>',
  ),
  undo: svg('<path d="M9 14L4 9l5-5"/><path d="M4 9h11a5 5 0 0 1 0 10h-3"/>'),
  retake: svg('<path d="M3 12a9 9 0 1 0 3-6.7L3 8"/><path d="M3 3v5h5"/>'),
  external: svg('<path d="M15 3h6v6"/><path d="M10 14L21 3"/><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/>'),
  sparkle: svg('<path d="M12 3l1.9 5.1L19 10l-5.1 1.9L12 17l-1.9-5.1L5 10l5.1-1.9z"/><path d="M19 15l.8 2.2L22 18l-2.2.8L19 21l-.8-2.2L16 18l2.2-.8z"/>'),
  check: svg('<path d="M20 6L9 17l-5-5"/>'),
  alert: svg('<circle cx="12" cy="12" r="9"/><path d="M12 8v4"/><path d="M12 16h.01"/>'),
};
