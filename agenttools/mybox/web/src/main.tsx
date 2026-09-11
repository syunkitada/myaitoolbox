import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { createBrowserRouter, RouterProvider } from 'react-router-dom'
import { getBasePath } from './api/client'
import App from './App'
import './monaco-setup'
import 'prismjs/themes/prism.css'
import './globals.css'

const router = createBrowserRouter([{ path: '*', element: <App /> }], { basename: getBasePath() })

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <RouterProvider router={router} />
  </StrictMode>,
)
