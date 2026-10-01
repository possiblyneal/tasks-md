import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

// `tasks api -web <dir>` serves the build output beside the JSON, so in
// production the client and the API share an origin and there is no CORS. The
// proxy below is what gives the dev server that same origin: without it every
// fetch during development would go to Vite's port and find no API there.
export default defineConfig({
  plugins: [react()],
  // Copied to the root of the build as they are: the manifest and the icons a
  // home screen reads by fixed names.
  publicDir: 'src/public',
  // The service worker is built beside the page rather than under `assets/`,
  // and unhashed, because its path is its scope and what the page registers.
  build: {
    rollupOptions: {
      input: { index: 'index.html', sw: 'src/sw.ts' },
      output: {
        entryFileNames: (chunk) =>
          chunk.name === 'sw' ? 'sw.js' : 'assets/[name]-[hash].js',
      },
    },
  },
  // 0.0.0.0 because the surface being proven here is a phone on the LAN
  // reaching this machine, which the default loopback bind refuses.
  server: {
    host: true,
    proxy: { '/api': 'http://localhost:8080' },
  },
  test: { environment: 'node' },
})
