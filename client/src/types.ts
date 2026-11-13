export type ReportKind = 'screenshot' | 'recording' | 'note';

export interface Rect {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface ConsoleEntry {
  level: 'error' | 'warn';
  message: string;
  time: string;
}

/** Technical context. Everything except `timestamp` is omitted when the user opts out. */
export interface ReportContext {
  timestamp: string;
  url?: string;
  title?: string;
  userAgent?: string;
  viewport?: { width: number; height: number; dpr: number };
  /** CSS selector of the element picked with the element tool. */
  selector?: string;
  /** Selected area in CSS pixels, relative to the viewport. */
  region?: Rect;
  console?: ConsoleEntry[];
  /** Extra key/values supplied by the host app (version, user, tenant…). */
  app?: Record<string, string>;
}

/** What gets sent to the backend (as the `payload` multipart field). */
export interface ReportPayload {
  title: string;
  description: string;
  kind: ReportKind;
  repo?: string;
  labels?: string[];
  context: ReportContext;
}

export interface Report {
  payload: ReportPayload;
  /** Annotated screenshot (PNG) or recording (WebM/MP4). */
  attachments: File[];
}

/** What the writing assistant gets: the notes so far, screenshots and (if allowed) context. */
export interface AssistRequest {
  title: string;
  description: string;
  kind: ReportKind;
  context: ReportContext;
  /** Annotated screenshots (WebP/PNG). */
  images: File[];
}

/** A suggested title and markdown description, for the reporter to review. */
export interface AssistDraft {
  title: string;
  description: string;
}

export interface SubmitResult {
  number?: number;
  url?: string;
}

export interface InspectOptions {
  /**
   * Base URL of the inspect server, e.g. `https://inspect.example.com`.
   * Reports are POSTed to `${endpoint}/v1/issues`. Not needed when `onSubmit` is given.
   */
  endpoint?: string;
  /** Target repository (`owner/name`). The server only accepts repos on its allowlist. */
  repo?: string;
  /** Labels to request. The server only keeps labels on its allowlist. */
  labels?: string[];
  /** Extra request headers, e.g. `{ Authorization: 'Bearer …' }`. Called on every submit. */
  headers?: () => Record<string, string> | Promise<Record<string, string>>;
  /** Host-app context to attach to every report (app version, user id, tenant…). */
  getContext?: () => Record<string, unknown> | Promise<Record<string, unknown>>;
  /** Replace the built-in HTTP submit, e.g. to route through your own API. */
  onSubmit?: (report: Report) => Promise<SubmitResult>;
  /**
   * Writing assistant ("Write it for me"). By default the button appears when the
   * inspect server has Azure OpenAI configured (GET /v1/capabilities). Pass a function
   * to use your own, or `false` to hide the button.
   */
  onAssist?: ((request: AssistRequest) => Promise<AssistDraft>) | false;
  /** Show the floating launcher button. Default `true`. */
  launcher?: boolean | { position?: 'bottom-right' | 'bottom-left' | 'top-right' | 'top-left' };
  /** Keyboard shortcut that opens the toolbar. Default `alt+shift+i`; `false` disables it. */
  shortcut?: string | false;
  /** Keep the last console errors/warnings and include them in reports. Default `true`. */
  captureConsole?: boolean;
  /** Maximum recording length in seconds. Default `120`. */
  maxRecordingSeconds?: number;
  /** Accent colour for buttons and highlights. Default `#6d4aff`. */
  accentColor?: string;
  /** z-index of the overlay. Default `2147483000`. */
  zIndex?: number;
}

export interface Inspect {
  /** Open the toolbar. */
  open(): void;
  /** Start a specific capture mode directly. */
  start(mode: 'region' | 'element' | 'record' | 'note'): void;
  /** Close everything and release the screen-capture stream. */
  close(): void;
  /** Remove all UI, listeners and console hooks. */
  destroy(): void;
}
