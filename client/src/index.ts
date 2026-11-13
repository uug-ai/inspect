import { Controller } from './controller';
import type { Inspect, InspectOptions } from './types';

export type {
  AssistDraft,
  AssistRequest,
  ConsoleEntry,
  Inspect,
  InspectOptions,
  Rect,
  Report,
  ReportContext,
  ReportKind,
  ReportPayload,
  SubmitResult,
} from './types';

/**
 * Mount the inspect UI (launcher button, toolbar, capture and report dialog).
 *
 * ```ts
 * const inspect = createInspect({ endpoint: 'https://inspect.example.com', repo: 'uug-ai/hub-frontend' });
 * // later: inspect.destroy();
 * ```
 */
export function createInspect(options: InspectOptions = {}): Inspect {
  if (typeof window === 'undefined' || typeof document === 'undefined') {
    // Server-side rendering: return an inert instance.
    const noop = () => {};
    return { open: noop, start: noop, close: noop, destroy: noop };
  }
  return new Controller(options);
}
