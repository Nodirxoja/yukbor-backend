import React from 'react'
import ReactDOM from 'react-dom/client'
import { Theme } from '@radix-ui/themes'
import '@radix-ui/themes/styles.css'
import './styles.css'
import App from './App'
import { Toaster } from './components/Toaster'

// panelBackground="translucent" is Radix's built-in glass surface: every
// Card/Table/panel gets a frosted, backdrop-blurred background (plan §11).
ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <Theme
      accentColor="blue"
      grayColor="slate"
      radius="large"
      panelBackground="translucent"
      hasBackground={false}
    >
      <Toaster>
        <App />
      </Toaster>
    </Theme>
  </React.StrictMode>,
)
