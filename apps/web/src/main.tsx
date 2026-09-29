import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import { App } from './App'
// PROTOTYPE — issue #99: `?variant=` swaps the list for the all-Repos variants.
import { AllRepos } from './prototype/AllRepos'
import './index.css'

const root = document.getElementById('root')
if (!root) throw new Error('index.html has no #root to mount on')

createRoot(root).render(
  <StrictMode>
    {new URLSearchParams(location.search).has('variant') ? (
      <AllRepos />
    ) : (
      <App />
    )}
  </StrictMode>,
)
