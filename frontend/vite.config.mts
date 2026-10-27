import react from '@vitejs/plugin-react'
import ViteYaml from '@modyfi/vite-plugin-yaml'
import { defineConfig } from 'vite'

export default defineConfig(() => {
  return {
    plugins: [
      react(),
      ViteYaml(),
    ],
    base: '',
    build: {
      emptyOutDir: true,

      // The browser this has to run in is the one on Windows 7, where the last
      // Chrome that will ever install is 109. Vite's default target is
      // baseline-widely-available — Chrome 111 — so by default the build was
      // allowed to emit syntax that a 109 cannot parse, and the page that used
      // it failed with a syntax error and no explanation.
      //
      // 87 rather than 109 on purpose: Windows 7 installs freeze wherever they
      // were when support ended, and a build that runs on 87 runs on all of
      // them. The syntax this costs is nothing on a bundle this size.
      //
      // Syntax is only half of it. Downlevelling cannot replace an API that
      // does not exist, so scripts/check-browser-support.mjs also fails the
      // build on anything newer than Chrome 109 — `Array.prototype.toSorted`,
      // which shipped in 110, was in the settings panel for exactly that
      // reason.
      target: ['chrome87', 'edge87', 'firefox78', 'safari14'],
    }
  }
})
