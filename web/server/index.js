// ServerDash's Node process: serves the built Svelte dashboard and proxies
// /api (including the WebSocket log stream) through to the Go backend, so
// the browser only ever talks to a single origin.
import express from 'express';
import { createProxyMiddleware } from 'http-proxy-middleware';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const distDir = path.join(__dirname, '..', 'dist');

const port = process.env.PORT || 9900;
const apiUrl = process.env.SERVERDASH_API_URL || 'http://127.0.0.1:8080';

const app = express();

// Mounted at the root (not app.use('/api', ...)) with a pathFilter instead,
// so Express never strips the '/api' prefix from req.url. That keeps the
// path identical on both the normal HTTP request path and the raw
// 'upgrade' event path used for the log-stream WebSocket below, which
// bypasses Express's router entirely.
const apiProxy = createProxyMiddleware({
	pathFilter: '/api',
	target: apiUrl,
	changeOrigin: true,
	ws: true,
});

app.use(apiProxy);
app.use(express.static(distDir));
app.get('*', (req, res) => {
	res.sendFile(path.join(distDir, 'index.html'));
});

const server = app.listen(port, () => {
	console.log(`serverdash web listening on :${port}, proxying /api to ${apiUrl}`);
});

// Express doesn't forward the raw 'upgrade' event on its own, so the log
// stream's WebSocket handshake has to be wired to the proxy manually.
server.on('upgrade', apiProxy.upgrade);
