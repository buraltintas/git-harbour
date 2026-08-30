import React from 'react';
import ReactDOM from 'react-dom/client';
import { BaseStyles, ThemeProvider } from '@primer/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import App from './App';
import './styles.css';
import './production.css';
import './target-game.css';
const q = new QueryClient();
ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ThemeProvider colorMode='auto'>
      <BaseStyles>
        <QueryClientProvider client={q}><App /></QueryClientProvider>
      </BaseStyles>
    </ThemeProvider>
  </React.StrictMode>,
);
