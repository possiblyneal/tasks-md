import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import { App } from './App'
// PROTOTYPE — issue #109: `?variant=` swaps the page for the phone board variants.
import { Phone } from './prototype/Phone'
import './index.css'

const root = document.getElementById('root')
if (!root) throw new Error('index.html has no #root to mount on')

createRoot(root).render(
  <StrictMode>
    {new URLSearchParams(location.search).has('variant') ? <Phone /> : <App />}
  </StrictMode>,
)
