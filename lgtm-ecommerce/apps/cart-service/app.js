const { api } = require('../common/traces');
const logger = require('../common/logger');


const express = require('express');
const app = express();
app.use(express.json());

const PORT = process.env.PORT || 8093;

// Redis setup
const { createClient } = require('redis');
const REDIS_HOST = process.env.REDIS_HOST || 'redis';
const redisClient = createClient({ url: `redis://${REDIS_HOST}:6379` });

let isRedisConnected = false;
async function initRedis() {
  try {
    await redisClient.connect();
    logger.info(`Cart-service connected to Redis cache at redis://${REDIS_HOST}:6379`);
    isRedisConnected = true;
  } catch (err) {
    logger.warn(`Redis Cache unavailable in cart-service, using fallback logic: ${err.message}`);
  }
}
initRedis();

// GET user cart
app.get('/cart/:userId', async (req, res) => {
  const { userId } = req.params;
  logger.info(`Fetching cart list for user ${userId}`);

  if (!isRedisConnected) {
    logger.warn('Redis Cache unavailable, returning empty mock cart fallback');
    return res.json({ user_id: userId, items: [], source: 'fallback-mock' });
  }

  const tracer = api.trace.getTracer('cart-tracer');
  return await tracer.startActiveSpan('GetRedisCart', async (span) => {
    span.setAttribute('redis.op', 'GET');
    span.setAttribute('user.id', userId);
    try {
      const val = await redisClient.get(`cart:${userId}`);
      const items = val ? JSON.parse(val) : [];
      logger.info(`Successfully retrieved cart with ${items.length} items from Redis`);
      return res.json({ user_id: userId, items, source: 'redis' });
    } catch (err) {
      span.recordException(err);
      span.setStatus({ code: api.SpanStatusCode.ERROR, message: err.message });
      logger.error(`Redis query failed for cart:${userId}`, { error: err.message });
      return res.status(500).json({ error: 'Redis failure' });
    } finally {
      span.end();
    }
  });
});

// POST user cart
app.post('/cart/:userId', async (req, res) => {
  const { userId } = req.params;
  const { items } = req.body || {};
  logger.info(`Saving cart list for user ${userId} with ${items?.length || 0} items`);

  if (!items || !Array.isArray(items)) {
    return res.status(400).json({ error: 'Items list must be a valid array' });
  }

  if (!isRedisConnected) {
    logger.warn('Redis Cache unavailable, mocking cart save response');
    return res.json({ status: 'saved', user_id: userId, items, source: 'fallback-mock' });
  }

  const tracer = api.trace.getTracer('cart-tracer');
  return await tracer.startActiveSpan('SetRedisCart', async (span) => {
    span.setAttribute('redis.op', 'SET');
    span.setAttribute('user.id', userId);
    try {
      await redisClient.set(`cart:${userId}`, JSON.stringify(items));
      logger.info(`Successfully saved cart list to Redis for user ${userId}`);
      return res.json({ status: 'saved', user_id: userId, items, source: 'redis' });
    } catch (err) {
      span.recordException(err);
      span.setStatus({ code: api.SpanStatusCode.ERROR, message: err.message });
      logger.error(`Redis write failed for cart:${userId}`, { error: err.message });
      return res.status(500).json({ error: 'Redis failure' });
    } finally {
      span.end();
    }
  });
});

app.get('/health', (req, res) => {
  return res.json({ status: 'healthy', service: 'cart-service', cache_connected: isRedisConnected });
});

app.listen(PORT, '0.0.0.0', () => {
  logger.info(`Cart-service listening on port ${PORT}`);
});
