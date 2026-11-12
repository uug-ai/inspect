export const styles = /* css */ `
:host {
  all: initial;
  --ix-accent: #6d4aff;
  --ix-accent-fg: #ffffff;
  --ix-danger: #e5484d;
  --ix-bg: #ffffff;
  --ix-bg-subtle: #f5f5f7;
  --ix-fg: #1c1c1f;
  --ix-fg-muted: #6b6b76;
  --ix-border: #e2e2e8;
  --ix-chrome: #18181b;
  --ix-chrome-fg: #f4f4f5;
  --ix-chrome-hover: #2e2e33;
  --ix-shadow: 0 12px 40px rgba(0, 0, 0, .28), 0 2px 8px rgba(0, 0, 0, .18);
  --ix-radius: 12px;
  font: 14px/1.45 system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
  color: var(--ix-fg);
}
@media (prefers-color-scheme: dark) {
  :host {
    --ix-bg: #1b1b1f;
    --ix-bg-subtle: #26262b;
    --ix-fg: #ececf1;
    --ix-fg-muted: #9d9daa;
    --ix-border: #34343b;
  }
}
*, *::before, *::after { box-sizing: border-box; }
button { font: inherit; color: inherit; cursor: pointer; }
.ix-icon { display: inline-flex; width: 18px; height: 18px; flex: none; }
.ix-icon svg { width: 100%; height: 100%; }
[hidden] { display: none !important; }

/* Launcher */
.ix-launcher {
  position: fixed; width: 44px; height: 44px; border-radius: 50%;
  border: 0; background: var(--ix-chrome); color: var(--ix-chrome-fg);
  display: grid; place-items: center; box-shadow: var(--ix-shadow);
  transition: transform .15s ease, background .15s ease; pointer-events: auto;
}
.ix-launcher:hover { transform: scale(1.06); background: var(--ix-accent); }
.ix-launcher .ix-icon { width: 22px; height: 22px; }
.ix-launcher[data-pos="bottom-right"] { right: 20px; bottom: 20px; }
.ix-launcher[data-pos="bottom-left"] { left: 20px; bottom: 20px; }
.ix-launcher[data-pos="top-right"] { right: 20px; top: 20px; }
.ix-launcher[data-pos="top-left"] { left: 20px; top: 20px; }

/* Toolbar */
.ix-toolbar {
  position: fixed; top: 16px; left: 50%; transform: translateX(-50%);
  display: flex; align-items: center; gap: 2px; padding: 5px;
  background: var(--ix-chrome); color: var(--ix-chrome-fg);
  border-radius: 999px; box-shadow: var(--ix-shadow); pointer-events: auto;
  animation: ix-drop .18s ease-out;
}
.ix-toolbar button {
  display: inline-flex; align-items: center; gap: 7px; height: 36px; padding: 0 14px;
  border: 0; border-radius: 999px; background: transparent; white-space: nowrap;
}
.ix-toolbar button:hover, .ix-toolbar button:focus-visible { background: var(--ix-chrome-hover); outline: none; }
.ix-toolbar button[data-mode="record"] .ix-icon { color: var(--ix-danger); }
.ix-toolbar .ix-sep { width: 1px; height: 20px; background: #3f3f46; margin: 0 4px; }
.ix-toolbar .ix-close { padding: 0; width: 36px; justify-content: center; }
.ix-toolbar kbd {
  font: 11px/1 ui-monospace, monospace; padding: 3px 5px; border-radius: 4px;
  background: #2e2e33; color: #a1a1aa;
}
@keyframes ix-drop { from { opacity: 0; transform: translate(-50%, -8px); } }
@media (max-width: 640px) { .ix-toolbar .ix-label, .ix-toolbar kbd { display: none; } }

/* Region / element selection */
.ix-select { position: fixed; inset: 0; cursor: crosshair; pointer-events: auto; }
.ix-select.ix-dim { background: rgba(10, 10, 14, .38); }
.ix-select.ix-picking { cursor: default; }
.ix-sel-box {
  position: fixed; border: 2px solid var(--ix-accent); border-radius: 2px;
  box-shadow: 0 0 0 100vmax rgba(10, 10, 14, .38); pointer-events: none;
}
.ix-pick-box {
  position: fixed; border: 2px solid var(--ix-accent); border-radius: 3px;
  background: color-mix(in srgb, var(--ix-accent) 14%, transparent);
  pointer-events: none; transition: all .06s linear;
}
.ix-tag {
  position: fixed; padding: 3px 8px; border-radius: 6px; font: 12px/1.3 ui-monospace, monospace;
  background: var(--ix-accent); color: var(--ix-accent-fg); pointer-events: none; white-space: nowrap;
  max-width: 60vw; overflow: hidden; text-overflow: ellipsis;
}
.ix-hint {
  position: fixed; top: 16px; left: 50%; transform: translateX(-50%);
  display: flex; align-items: center; gap: 10px; padding: 8px 8px 8px 16px;
  background: var(--ix-chrome); color: var(--ix-chrome-fg); border-radius: 999px;
  box-shadow: var(--ix-shadow); pointer-events: auto; white-space: nowrap; cursor: default;
}
.ix-hint button {
  height: 28px; padding: 0 12px; border-radius: 999px; border: 0;
  background: var(--ix-chrome-hover); color: var(--ix-chrome-fg);
}
.ix-hint button.ix-primary { background: var(--ix-accent); color: var(--ix-accent-fg); }
.ix-hint kbd { font: 11px/1 ui-monospace, monospace; padding: 3px 5px; border-radius: 4px; background: #2e2e33; color: #a1a1aa; }

/* Recording */
.ix-rec-frame {
  position: fixed; outline: 2px dashed var(--ix-danger); outline-offset: 3px; pointer-events: none;
}
.ix-rec-bar {
  position: fixed; display: flex; align-items: center; gap: 10px; padding: 5px 5px 5px 14px;
  background: var(--ix-chrome); color: var(--ix-chrome-fg); border-radius: 999px;
  box-shadow: var(--ix-shadow); pointer-events: auto; font-variant-numeric: tabular-nums;
}
.ix-rec-dot { width: 10px; height: 10px; border-radius: 50%; background: var(--ix-danger); animation: ix-pulse 1.2s infinite; }
@keyframes ix-pulse { 50% { opacity: .35; } }
.ix-rec-bar button {
  display: inline-flex; align-items: center; gap: 6px; height: 30px; padding: 0 12px;
  border: 0; border-radius: 999px; background: var(--ix-danger); color: #fff;
}
.ix-rec-bar button.ix-ghost { background: var(--ix-chrome-hover); }
.ix-rec-bar .ix-icon { width: 14px; height: 14px; }

/* Busy indicator while the browser asks for capture permission */
.ix-busy {
  position: fixed; top: 16px; left: 50%; transform: translateX(-50%);
  padding: 9px 16px; border-radius: 999px; background: var(--ix-chrome); color: var(--ix-chrome-fg);
  box-shadow: var(--ix-shadow); pointer-events: auto;
}

/* Report dialog */
.ix-backdrop {
  position: fixed; inset: 0; background: rgba(10, 10, 14, .55); pointer-events: auto;
  display: grid; place-items: center; padding: 24px; animation: ix-fade .15s ease-out;
}
@keyframes ix-fade { from { opacity: 0; } }
.ix-dialog {
  width: min(1200px, 100%); max-height: calc(100vh - 48px); display: grid;
  grid-template-columns: minmax(0, 1fr) 340px; background: var(--ix-bg); color: var(--ix-fg);
  border-radius: var(--ix-radius); box-shadow: var(--ix-shadow); overflow: hidden;
}
.ix-dialog.ix-note { grid-template-columns: 1fr; width: min(480px, 100%); }
@media (max-width: 860px) {
  .ix-backdrop { padding: 0; }
  .ix-dialog { grid-template-columns: 1fr; max-height: 100vh; height: 100%; border-radius: 0; overflow: auto; }
}
.ix-stage { display: flex; flex-direction: column; min-height: 0; background: var(--ix-bg-subtle); border-right: 1px solid var(--ix-border); }
.ix-canvas-wrap {
  position: relative; flex: 1; min-height: 240px; display: grid; place-items: center; padding: 16px; overflow: hidden;
  background-image: conic-gradient(var(--ix-border) 25%, transparent 0 50%, var(--ix-border) 0 75%, transparent 0);
  background-size: 16px 16px; background-color: var(--ix-bg-subtle);
}
.ix-canvas-wrap canvas, .ix-canvas-wrap video {
  max-width: 100%; max-height: calc(100vh - 160px); border-radius: 4px;
  box-shadow: 0 1px 4px rgba(0, 0, 0, .25); background: #000;
}
.ix-canvas-wrap canvas { cursor: crosshair; touch-action: none; }
.ix-text-input {
  position: absolute; min-width: 120px; padding: 2px 4px; border: 1px dashed currentColor;
  background: rgba(255, 255, 255, .9); font: 600 16px system-ui, sans-serif; outline: none;
}

.ix-tools {
  display: flex; align-items: center; gap: 4px; padding: 8px 12px; flex-wrap: wrap;
  border-bottom: 1px solid var(--ix-border); background: var(--ix-bg);
}
.ix-tools button {
  display: inline-grid; place-items: center; width: 34px; height: 34px; border-radius: 8px;
  border: 1px solid transparent; background: transparent; color: var(--ix-fg-muted);
}
.ix-tools button:hover { background: var(--ix-bg-subtle); color: var(--ix-fg); }
.ix-tools button[aria-pressed="true"] { background: color-mix(in srgb, var(--ix-accent) 14%, transparent); color: var(--ix-accent); border-color: color-mix(in srgb, var(--ix-accent) 40%, transparent); }
.ix-tools button:disabled { opacity: .35; cursor: default; background: transparent; }
.ix-tools .ix-sep { width: 1px; height: 22px; background: var(--ix-border); margin: 0 6px; }
.ix-tools .ix-spacer { flex: 1; }
.ix-swatch {
  width: 22px !important; height: 22px !important; border-radius: 50% !important; padding: 0;
  border: 2px solid var(--ix-bg) !important; box-shadow: 0 0 0 1px var(--ix-border);
}
.ix-swatch[aria-pressed="true"] { box-shadow: 0 0 0 2px var(--ix-fg); }
.ix-tools .ix-text-btn { width: auto; padding: 0 10px; gap: 6px; display: inline-flex; font-size: 13px; }

.ix-form { display: flex; flex-direction: column; gap: 14px; padding: 20px; overflow: auto; }
.ix-form h2 { margin: 0; font-size: 17px; font-weight: 650; }
.ix-form p.ix-sub { margin: -8px 0 0; color: var(--ix-fg-muted); font-size: 13px; }
.ix-field { display: flex; flex-direction: column; gap: 6px; }
.ix-field label { font-size: 13px; font-weight: 550; }
.ix-field input, .ix-field textarea {
  font: inherit; color: var(--ix-fg); background: var(--ix-bg); width: 100%;
  border: 1px solid var(--ix-border); border-radius: 8px; padding: 8px 10px; outline: none;
}
.ix-field textarea { min-height: 140px; resize: vertical; }
.ix-field input:focus, .ix-field textarea:focus {
  border-color: var(--ix-accent); box-shadow: 0 0 0 3px color-mix(in srgb, var(--ix-accent) 22%, transparent);
}
.ix-context { border: 1px solid var(--ix-border); border-radius: 8px; font-size: 13px; }
.ix-context summary { padding: 8px 10px; cursor: pointer; display: flex; align-items: center; gap: 8px; list-style: none; }
.ix-context summary::-webkit-details-marker { display: none; }
.ix-context summary input { margin: 0; accent-color: var(--ix-accent); }
.ix-context summary span { flex: 1; }
.ix-context summary em { font-style: normal; color: var(--ix-fg-muted); }
.ix-context dl { margin: 0; padding: 0 10px 10px; display: grid; grid-template-columns: auto 1fr; gap: 4px 12px; }
.ix-context dt { color: var(--ix-fg-muted); }
.ix-context dd { margin: 0; overflow-wrap: anywhere; font-family: ui-monospace, monospace; font-size: 12px; }
.ix-error {
  display: flex; gap: 8px; align-items: flex-start; padding: 10px; border-radius: 8px;
  background: color-mix(in srgb, var(--ix-danger) 12%, transparent); color: var(--ix-danger); font-size: 13px;
}
.ix-actions { display: flex; gap: 8px; justify-content: flex-end; margin-top: auto; padding-top: 4px; }
.ix-btn {
  display: inline-flex; align-items: center; justify-content: center; gap: 6px;
  height: 36px; padding: 0 16px; border-radius: 8px; border: 1px solid var(--ix-border);
  background: var(--ix-bg); color: var(--ix-fg); font-weight: 550;
}
.ix-btn:hover { background: var(--ix-bg-subtle); }
.ix-btn.ix-primary { background: var(--ix-accent); border-color: var(--ix-accent); color: var(--ix-accent-fg); }
.ix-btn.ix-primary:hover { filter: brightness(1.08); }
.ix-btn:disabled { opacity: .6; cursor: progress; }
.ix-spin { width: 14px; height: 14px; border: 2px solid currentColor; border-right-color: transparent; border-radius: 50%; animation: ix-rot .7s linear infinite; }
@keyframes ix-rot { to { transform: rotate(360deg); } }

/* Toast */
.ix-toast {
  position: fixed; bottom: 20px; left: 50%; transform: translateX(-50%);
  display: flex; align-items: center; gap: 10px; padding: 10px 10px 10px 14px; border-radius: 10px;
  background: var(--ix-chrome); color: var(--ix-chrome-fg); box-shadow: var(--ix-shadow); pointer-events: auto;
  animation: ix-rise .2s ease-out;
}
.ix-toast .ix-icon { color: #4ade80; }
.ix-toast.ix-toast-error .ix-icon { color: var(--ix-danger); }
.ix-toast.ix-toast-busy { padding-right: 16px; }
.ix-toast .ix-spin { flex: none; color: #a1a1aa; }
.ix-toast a { color: inherit; display: inline-flex; align-items: center; gap: 4px; font-weight: 600; }
.ix-toast a .ix-icon { color: inherit; width: 14px; height: 14px; }
.ix-toast button { border: 0; background: transparent; color: #a1a1aa; display: grid; place-items: center; padding: 4px; border-radius: 6px; }
.ix-toast button:hover { background: var(--ix-chrome-hover); }
@keyframes ix-rise { from { opacity: 0; transform: translate(-50%, 8px); } }
`;
