import { resolve } from 'node:path';
import { defineConfig } from 'vite';

export default defineConfig({
  build: {
    outDir: resolve(__dirname, 'dist'),
    emptyOutDir: true,
    sourcemap: true,
    lib: {
      entry: resolve(__dirname, 'src/index.ts'),
      name: 'UugInspect',
      fileName: 'inspect',
      formats: ['es', 'umd'],
    },
  },
  server: {
    port: 5180,
  },
});
