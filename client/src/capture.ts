import type { Rect } from './types';

type FrameVideo = HTMLVideoElement & {
  requestVideoFrameCallback?: (cb: () => void) => number;
};

const nextFrame = () => new Promise<void>((r) => requestAnimationFrame(() => r()));

export class CaptureUnsupportedError extends Error {}

/**
 * Wraps one `getDisplayMedia` stream of (ideally) the current tab. The stream is kept
 * open while the toolbar is open so the user is asked for permission once per session.
 */
export class ScreenSession {
  private stream: MediaStream | null = null;
  private video: FrameVideo | null = null;

  constructor(private readonly onEnded: () => void) {}

  get active(): boolean {
    return !!this.stream;
  }

  static supported(): boolean {
    return !!navigator.mediaDevices?.getDisplayMedia;
  }

  async ensure(): Promise<void> {
    if (this.stream) return;
    if (!ScreenSession.supported()) {
      throw new CaptureUnsupportedError('Screen capture is not supported in this browser.');
    }
    const stream = await navigator.mediaDevices.getDisplayMedia({
      video: { displaySurface: 'browser', frameRate: 30 },
      audio: false,
      // Chromium-only hints: offer "this tab" first and don't let the user switch mid-way.
      preferCurrentTab: true,
      selfBrowserSurface: 'include',
      surfaceSwitching: 'exclude',
    } as DisplayMediaStreamOptions);

    const video = document.createElement('video') as FrameVideo;
    video.muted = true;
    video.playsInline = true;
    video.srcObject = stream;
    await video.play();
    if (!video.videoWidth) {
      await new Promise<void>((r) => video.addEventListener('loadedmetadata', () => r(), { once: true }));
    }

    stream.getVideoTracks()[0]?.addEventListener('ended', () => {
      this.release();
      this.onEnded();
    });
    this.stream = stream;
    this.video = video;
  }

  /**
   * True when the shared surface is this tab, so viewport coordinates map onto the frame.
   * If the user shared another tab, a window or a screen we fall back to the full frame.
   */
  private mapsToViewport(): boolean {
    const track = this.stream?.getVideoTracks()[0];
    const settings = track?.getSettings() as (MediaTrackSettings & { displaySurface?: string }) | undefined;
    if (settings?.displaySurface && settings.displaySurface !== 'browser') return false;
    const v = this.video!;
    const frameAspect = v.videoWidth / v.videoHeight;
    const viewAspect = window.innerWidth / window.innerHeight;
    return Math.abs(frameAspect - viewAspect) / viewAspect < 0.05;
  }

  /** Map a viewport rect onto the video frame (or the whole frame if we can't). */
  private sourceRect(region?: Rect): Rect {
    const v = this.video!;
    const full = { x: 0, y: 0, width: v.videoWidth, height: v.videoHeight };
    if (!region || !this.mapsToViewport()) return full;
    const sx = v.videoWidth / window.innerWidth;
    const sy = v.videoHeight / window.innerHeight;
    const x = Math.max(0, Math.round(region.x * sx));
    const y = Math.max(0, Math.round(region.y * sy));
    return {
      x,
      y,
      width: Math.max(1, Math.min(v.videoWidth - x, Math.round(region.width * sx))),
      height: Math.max(1, Math.min(v.videoHeight - y, Math.round(region.height * sy))),
    };
  }

  /** Wait until the video has rendered frames painted after our overlay was hidden. */
  async settle(): Promise<void> {
    await nextFrame();
    await nextFrame();
    const v = this.video!;
    const waitFrame = () =>
      new Promise<void>((resolve) => {
        const timer = setTimeout(resolve, 250);
        if (v.requestVideoFrameCallback) {
          v.requestVideoFrameCallback(() => {
            clearTimeout(timer);
            resolve();
          });
        }
      });
    await waitFrame();
    await waitFrame();
  }

  grab(region?: Rect): HTMLCanvasElement {
    if (!this.video) throw new Error('No active capture');
    const src = this.sourceRect(region);
    const canvas = document.createElement('canvas');
    canvas.width = src.width;
    canvas.height = src.height;
    canvas.getContext('2d')!.drawImage(this.video, src.x, src.y, src.width, src.height, 0, 0, src.width, src.height);
    return canvas;
  }

  /** Record the stream, cropped to `region`. Resolves with the video when `stop()` is called. */
  record(region: Rect | undefined): Recording {
    if (!this.video || !this.stream) throw new Error('No active capture');
    const src = this.sourceRect(region);
    const v = this.video;
    const full = src.x === 0 && src.y === 0 && src.width === v.videoWidth && src.height === v.videoHeight;

    let stream = this.stream;
    let stopDrawing = () => {};
    if (!full) {
      // Crop by drawing each frame into a canvas and recording the canvas.
      const canvas = document.createElement('canvas');
      canvas.width = src.width - (src.width % 2);
      canvas.height = src.height - (src.height % 2);
      const ctx = canvas.getContext('2d')!;
      let running = true;
      const draw = () => {
        if (!running) return;
        ctx.drawImage(v, src.x, src.y, canvas.width, canvas.height, 0, 0, canvas.width, canvas.height);
        if (v.requestVideoFrameCallback) v.requestVideoFrameCallback(draw);
        else requestAnimationFrame(draw);
      };
      draw();
      stopDrawing = () => {
        running = false;
      };
      stream = canvas.captureStream(30);
    }
    return new Recording(stream, stopDrawing);
  }

  release(): void {
    this.stream?.getTracks().forEach((t) => t.stop());
    if (this.video) this.video.srcObject = null;
    this.stream = null;
    this.video = null;
  }
}

const MIME_CANDIDATES = ['video/webm;codecs=vp9', 'video/webm;codecs=vp8', 'video/webm', 'video/mp4'];

export class Recording {
  readonly mimeType: string;
  private recorder: MediaRecorder;
  private chunks: Blob[] = [];
  private done: Promise<Blob>;

  constructor(stream: MediaStream, private readonly cleanup: () => void) {
    this.mimeType = MIME_CANDIDATES.find((m) => MediaRecorder.isTypeSupported(m)) ?? '';
    this.recorder = new MediaRecorder(stream, this.mimeType ? { mimeType: this.mimeType, videoBitsPerSecond: 4_000_000 } : undefined);
    this.recorder.ondataavailable = (e) => e.data.size && this.chunks.push(e.data);
    this.done = new Promise((resolve) => {
      this.recorder.onstop = () => {
        this.cleanup();
        resolve(new Blob(this.chunks, { type: (this.recorder.mimeType || this.mimeType || 'video/webm').split(';')[0] }));
      };
    });
    this.recorder.start(1000);
  }

  stop(): Promise<Blob> {
    if (this.recorder.state !== 'inactive') this.recorder.stop();
    return this.done;
  }
}
