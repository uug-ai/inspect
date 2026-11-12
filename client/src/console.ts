import type { ConsoleEntry } from './types';

const LIMIT = 50;
const MAX_MESSAGE = 2000;

function stringify(value: unknown): string {
  if (value instanceof Error) return value.stack || `${value.name}: ${value.message}`;
  if (typeof value === 'string') return value;
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

/** Keeps a ring buffer of recent console errors/warnings and uncaught errors. */
export class ConsoleBuffer {
  private entries: ConsoleEntry[] = [];
  private restore: Array<() => void> = [];

  install(): void {
    for (const level of ['error', 'warn'] as const) {
      const original = console[level];
      console[level] = (...args: unknown[]) => {
        this.push(level, args.map(stringify).join(' '));
        original.apply(console, args);
      };
      this.restore.push(() => {
        console[level] = original;
      });
    }
    const onError = (e: ErrorEvent) => this.push('error', e.error ? stringify(e.error) : e.message);
    const onRejection = (e: PromiseRejectionEvent) =>
      this.push('error', `Unhandled rejection: ${stringify(e.reason)}`);
    window.addEventListener('error', onError);
    window.addEventListener('unhandledrejection', onRejection);
    this.restore.push(() => {
      window.removeEventListener('error', onError);
      window.removeEventListener('unhandledrejection', onRejection);
    });
  }

  uninstall(): void {
    this.restore.forEach((fn) => fn());
    this.restore = [];
  }

  snapshot(): ConsoleEntry[] {
    return this.entries.slice();
  }

  private push(level: ConsoleEntry['level'], message: string): void {
    this.entries.push({ level, message: message.slice(0, MAX_MESSAGE), time: new Date().toISOString() });
    if (this.entries.length > LIMIT) this.entries.shift();
  }
}
