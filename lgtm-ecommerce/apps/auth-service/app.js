const { api } = require('../common/traces');
const logger = require('../common/logger');


const express = require('express');
const app = express();
app.use(express.json());

const PORT = process.env.PORT || 8091;

// Mock Login Route
app.post('/login', (req, res) => {
  const { username, password } = req.body || {};
  logger.info(`Processing mock login for user '${username}'`);

  if (!username || !password) {
    logger.warn('Login request rejected: missing username or password');
    return res.status(400).json({ error: 'Username and password required' });
  }

  // Generates a mock token
  const token = `Bearer token-for-${username}`;
  logger.info(`Login successful for '${username}', returned generated token`);
  return res.json({ status: 'authenticated', token, user_id: `user_${username.toLowerCase()}` });
});

// Trace context-preserving verify token endpoint
app.post('/verify', (req, res) => {
  const { token } = req.body || {};
  logger.info(`Verifying token: ${token}`);

  const tracer = api.trace.getTracer('auth-tracer');
  const activeSpan = api.trace.getActiveSpan();
  
  if (activeSpan) {
    activeSpan.setAttribute('auth.token', token || 'empty');
  }

  if (!token || !token.startsWith('Bearer ')) {
    logger.warn('Authentication validation check failed: missing or malformed token');
    return res.status(401).json({ verified: false, error: 'Malformed authorization token' });
  }

  const username = token.replace('Bearer token-for-', '').replace('Bearer ', '');
  const userId = `user_${username.toLowerCase() || 'anonymous'}`;
  
  logger.info(`Token verified successfully. Identified user: ${userId}`);
  return res.json({ verified: true, user_id: userId });
});

app.get('/health', (req, res) => {
  return res.json({ status: 'healthy', service: 'auth-service' });
});

app.listen(PORT, '0.0.0.0', () => {
  logger.info(`Auth-service listening on port ${PORT}`);
});

