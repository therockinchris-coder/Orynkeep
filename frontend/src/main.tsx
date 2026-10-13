import { StrictMode, useEffect, useState } from 'react';
import { createRoot } from 'react-dom/client';
import './styles.css';
import { Setup } from './Setup';

type Status = 'checking' | 'ready' | 'unavailable';

function App() {
  const [status, setStatus] = useState<Status>('checking');
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    let active = true;
    setStatus('checking');
    const timeout = window.setTimeout(() => controller.abort(), 5000);
    fetch('/api/health/ready', { signal: controller.signal, cache: 'no-store' })
      .then(async response => {
        if (!response.ok) throw new Error('Unavailable');
        const result: unknown = await response.json();
        if (!result || typeof result !== 'object' || !('status' in result) || result.status !== 'ready') {
          throw new Error('Unexpected response');
        }
        if (active) setStatus('ready');
      })
      .catch(() => { if (active) setStatus('unavailable'); })
      .finally(() => window.clearTimeout(timeout));
    return () => { active = false; controller.abort(); window.clearTimeout(timeout); };
  }, [attempt]);

  return <div className="shell">
    <header><a className="brand" href="/">Orynkeep<span>YOUR PERSONAL CLOUD</span></a><span className="badge">v0.1 · in development</span></header>
    <main>
      <p className="eyebrow">A place for everything you keep.</p>
      <h1>Your files.<br />Your server.</h1>
      <p className="intro">A self-hosted home for your files, documents, and photographs.</p>
      <section className="health" aria-labelledby="health-title">
        <div><h2 id="health-title">System connection</h2><p role="status" aria-live="polite">{status === 'ready' ? 'Backend, database, and storage are ready.' : status === 'checking' ? 'Checking your server…' : 'Unable to reach a ready server. Check the service logs and retry.'}</p></div>
        <button disabled={status === 'checking'} onClick={() => setAttempt(value => value + 1)}>Check again</button>
      </section>
      <Setup />
      <div className="areas">
        {[
          ['01', 'Files', 'Your folders and files, kept on your own storage.'],
          ['02', 'Office', 'A simple space to write and save documents.'],
          ['03', 'Photos', 'A gallery for the moments you want to keep.'],
        ].map(([number, title, description]) => <section className="area" key={title}>
          <span className="number">{number}</span><h2>{title}</h2><p>{description}</p><span className="planned">Planned for v0.1</span>
        </section>)}
      </div>
      <p className="note">The foundation is being built. File uploads, the document editor, and the photo gallery will arrive in the next development stages.</p>
    </main>
    <footer>Orynkeep · Self-hosted, by design.</footer>
  </div>;
}

createRoot(document.getElementById('root')!).render(<StrictMode><App /></StrictMode>);
