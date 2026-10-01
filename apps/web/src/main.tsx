import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import { App } from './App'
import './index.css'

const root = document.getElementById('root')
if (!root) throw new Error('index.html has no #root to mount on')

createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>,
)

// Only a build has a worker to register; the dev server serves no `/sw.js`.
// It is a module because it shares `offline.ts` with the page, which Safari
// runs from iOS 15.
if (import.meta.env.PROD) {
  void navigator.serviceWorker?.register('/sw.js', { type: 'module' })
}
