import { useEffect, useState, type FormEvent } from 'react';

type SetupStatus = { complete: boolean; enabled: boolean };

export function Setup() {
  const [status, setStatus] = useState<SetupStatus | null>(null);
  const [email, setEmail] = useState('');
  const [token, setToken] = useState('');
  const [password, setPassword] = useState('');
  const [confirmation, setConfirmation] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let active = true;
    const controller = new AbortController();
    const timer = window.setTimeout(() => controller.abort(), 8000);
    setError('');
    fetch('/api/setup/status', { signal: controller.signal, cache: 'no-store' })
      .then(async response => {
        if (!response.ok) throw new Error('Unable to check setup. Check the server and retry.');
        const data: unknown = await response.json();
        if (!data || typeof data !== 'object' || !('complete' in data) || !('enabled' in data) || typeof data.complete !== 'boolean' || typeof data.enabled !== 'boolean') {
          throw new Error('Unexpected setup response. Check that the backend has been updated.');
        }
        if (active) setStatus({ complete: data.complete, enabled: data.enabled });
      })
      .catch(error => { if (active) setError(error instanceof Error && error.name !== 'AbortError' ? error.message : 'The setup check timed out. Please retry.'); })
      .finally(() => window.clearTimeout(timer));
    return () => { active = false; controller.abort(); window.clearTimeout(timer); };
  }, [attempt]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError('');
    if (password !== confirmation) { setError('Your passwords do not match.'); return; }
    if ([...password].length < 12 || new TextEncoder().encode(password).length > 1024) { setError('Use at least 12 characters and no more than 1024 bytes.'); return; }
    setBusy(true);
    const controller = new AbortController();
    const timer = window.setTimeout(() => controller.abort(), 20000);
    try {
      const response = await fetch('/api/setup', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-Setup-Token': token },
        body: JSON.stringify({ email, password }),
        signal: controller.signal,
      });
      const data: unknown = await response.json();
      if (response.status === 409) {
        setStatus({ complete: true, enabled: false });
        setToken(''); setPassword(''); setConfirmation('');
        return;
      }
      if (!response.ok) {
        throw new Error(data && typeof data === 'object' && 'error' in data && typeof data.error === 'string' ? data.error : 'Unable to create the administrator account.');
      }
      if (!data || typeof data !== 'object' || !('complete' in data) || data.complete !== true) {
        throw new Error('Unexpected setup response. Reload to check whether setup completed.');
      }
      setStatus({ complete: true, enabled: false });
      setToken(''); setPassword(''); setConfirmation('');
    } catch (error) {
      setError(error instanceof Error && error.name !== 'AbortError' ? error.message : 'The request timed out. Reload to check whether setup completed before trying again.');
    } finally { window.clearTimeout(timer); setBusy(false); }
  }

  if (!status) return <section className="setup-panel" aria-labelledby="setup-heading">
    <h2 id="setup-heading">First-run setup</h2>
    {error ? <><p role="alert">{error}</p><button onClick={() => setAttempt(value => value + 1)}>Retry setup check</button></> : <p role="status">Checking installation…</p>}
  </section>;

  if (status.complete) return <section className="setup-panel complete" aria-labelledby="setup-heading">
    <p className="eyebrow">Setup complete</p><h2 id="setup-heading">Your administrator account is ready.</h2>
    <p>First-run setup is now closed. Your account is stored securely in the database.</p>
    <p className="note">Login and sessions are the next development step. This page does not sign you in.</p>
  </section>;

  if (!status.enabled) return <section className="setup-panel" aria-labelledby="setup-heading">
    <h2 id="setup-heading">Enable first-run setup</h2><p>Add a setup token to your server configuration, then restart the backend. Your installation cannot be claimed until you enable setup.</p>
    {error && <p className="form-error" role="alert">{error}</p>}
    <button onClick={() => setAttempt(value => value + 1)}>Check again</button>
  </section>;

  return <section className="setup-panel" aria-labelledby="setup-heading">
    <p className="eyebrow">Welcome to your personal cloud</p>
    <h2 id="setup-heading">Create your administrator account</h2>
    <p>This account will manage your Orynkeep installation. Setup can only be completed once.</p>
    <form onSubmit={submit} aria-busy={busy}>
      <fieldset disabled={busy}>
        <label htmlFor="setup-token">Setup token</label>
        <input id="setup-token" type="password" value={token} onChange={event => setToken(event.target.value)} required minLength={32} maxLength={512} autoComplete="off" spellCheck={false} aria-describedby="token-help" />
        <small id="token-help">Enter the setup token from your server configuration.</small>
        <label htmlFor="admin-email">Email address</label>
        <input id="admin-email" type="email" value={email} onChange={event => setEmail(event.target.value)} required maxLength={254} autoComplete="username" />
        <label htmlFor="admin-password">Password</label>
        <input id="admin-password" type="password" value={password} onChange={event => setPassword(event.target.value)} required maxLength={1024} autoComplete="new-password" aria-describedby="password-help" />
        <small id="password-help">Use at least 12 characters. A long, unique passphrase works well.</small>
        <label htmlFor="admin-confirm">Confirm password</label>
        <input id="admin-confirm" type="password" value={confirmation} onChange={event => setConfirmation(event.target.value)} required maxLength={1024} autoComplete="new-password" />
        {error && <p className="form-error" role="alert">{error}</p>}
        <button type="submit">{busy ? 'Creating your account…' : 'Create administrator account'}</button>
      </fieldset>
    </form>
  </section>;
}
