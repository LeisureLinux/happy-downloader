import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App } from './App'

import '@fontsource/roboto/300.css'
import '@fontsource/roboto/400.css'
import '@fontsource/roboto/500.css'
import '@fontsource/roboto/700.css'

// Explicit weights rather than the package's index: the bare specifier has no
// type declaration, so `tsc --noEmit` fails on it. 400 for the argument
// previews, 600 for the log panel.
import '@fontsource/roboto-mono/400.css'
import '@fontsource/roboto-mono/600.css'

const root = createRoot(document.getElementById('root')!)

root.render(
  <StrictMode>
    <App />
  </StrictMode>
)
