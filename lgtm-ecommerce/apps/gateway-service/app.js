const { api, getActiveExemplar, setBaggageValue, attachBaggageToSpan } = require('../common/traces');
const { calculationCounter, errorCounter, checkoutRevenueCounter } = require('../common/metrics');
const logger = require('../common/logger');




const Hapi = require('@hapi/hapi');
const Inert = require('@hapi/inert');
const Boom = require('@hapi/boom');

// Downstream service base URLs from Docker Compose environment settings
const AUTH_SERVICE_URL = process.env.AUTH_SERVICE_URL || 'http://auth-service:8091';
const CATALOG_SERVICE_URL = process.env.CATALOG_SERVICE_URL || 'http://catalog-service:8092';
const CART_SERVICE_URL = process.env.CART_SERVICE_URL || 'http://cart-service:8093';
const ORDER_SERVICE_URL = process.env.ORDER_SERVICE_URL || 'http://order-service:8094';

async function fetchJson(url, options = {}) {
  const headers = options.headers ? { ...options.headers } : {};
  // Inject trace context headers automatically for distributed tracing
  const { W3CTraceContextPropagator } = require('@opentelemetry/core');
  const propagator = new W3CTraceContextPropagator();
  propagator.inject(api.context.active(), headers, {
    set: (carrier, key, value) => {
      carrier[key] = value;
    }
  });

  const response = await fetch(url, {
    keepalive: true,
    ...options,
    headers
  });
  let data;
  try {
    data = await response.json();
  } catch (err) {
    const text = await response.text().catch(() => '');
    data = { error: text || `HTTP ${response.status}` };
  }
  return { status: response.status, data };
}

const VALID_API_KEY = process.env.API_KEY || 'lgtm-secret-key';
const PORT = process.env.PORT || 8090;

const init = async () => {
  const server = Hapi.server({
    port: PORT,
    host: '0.0.0.0'
  });

  await server.register(Inert);

  // Global Auth Ext Check
  server.ext('onPreHandler', (request, h) => {
    if (request.route.settings.plugins.skipAuth) {
      return h.continue;
    }
    const rawApiKey = request.headers['x-api-key'] || request.query.api_key;
    const apiKey = typeof rawApiKey === 'string' ? rawApiKey.split(',')[0].trim() : undefined;

    if (!apiKey || apiKey !== VALID_API_KEY) {
      logger.warn(`Unauthorized access attempt to: ${request.path}`);
      throw Boom.unauthorized('Unauthorized: Invalid or missing X-API-Key header.');
    }
    return h.continue;
  });

  // 1. Diagnostics Health
  server.route({
    method: 'GET',
    path: '/health',
    options: { plugins: { skipAuth: true } },
    handler: () => ({ status: 'healthy', service: 'gateway-service' })
  });

  // 3. User Authentication Validation Route
  server.route({
    method: 'POST',
    path: '/login',
    options: { plugins: { skipAuth: true } },
    handler: async (request, h) => {
      logger.info('Gateway routing login credentials to auth-service');
      const { status, data } = await fetchJson(`${AUTH_SERVICE_URL}/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(request.payload)
      });
      return h.response(data).code(status);
    }
  });

  // 4. E-Commerce Catalog Route (Go Backend)
  server.route({
    method: 'GET',
    path: '/catalog',
    options: { plugins: { skipAuth: true } },
    handler: async (request, h) => {
      logger.info('Gateway routing catalog list fetch request downstream');
      const { status, data } = await fetchJson(`${CATALOG_SERVICE_URL}/catalog`);
      return h.response(data).code(status);
    }
  });

  // 5. Redis Cart Operations Route
  server.route({
    method: 'GET',
    path: '/cart/{userId}',
    handler: async (request, h) => {
      const { userId } = request.params;
      logger.info(`Gateway proxying cart fetch request for user ${userId}`);
      const { status, data } = await fetchJson(`${CART_SERVICE_URL}/cart/${userId}`);
      return h.response(data).code(status);
    }
  });

  server.route({
    method: 'POST',
    path: '/cart/{userId}',
    handler: async (request, h) => {
      const { userId } = request.params;
      logger.info(`Gateway updating cart items list for user ${userId}`);
      const { status, data } = await fetchJson(`${CART_SERVICE_URL}/cart/${userId}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(request.payload)
      });
      return h.response(data).code(status);
    }
  });

  // 6. E-Commerce Order placement Checkout (Deep Cascade Cascade Route)
  server.route({
    method: 'POST',
    path: '/checkout',
    handler: async (request, h) => {
      logger.info('Gateway starting deep checkout cascade transaction flow');
      calculationCounter.add(1, { number: 'checkout' });

      // Determine customer tier based on Authorization token
      const tokenVal = request.headers['authorization'] || 'Bearer token-for-Alice';
      const username = tokenVal.replace('Bearer token-for-', '').replace('Bearer ', '');
      const tier = ['Alice', 'Bob'].includes(username) ? 'gold' : 'regular';

      // Set baggage context
      const ctxWithBaggage = setBaggageValue('customer.tier', tier);

      return await api.context.with(ctxWithBaggage, async () => {
        const tracer = api.trace.getTracer('gateway-tracer');
        return await tracer.startActiveSpan('GatewayCheckoutWorkflow', async (span) => {
          attachBaggageToSpan(span);
          span.setAttribute('checkout.quantity', request.payload.quantity || 1);

          try {
            // Token check verify downstream
            const { status: authStatus, data: authData } = await fetchJson(`${AUTH_SERVICE_URL}/verify`, {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ token: tokenVal })
            });

            if (authStatus !== 200 || !authData.verified) {
              logger.warn('Auth token verification verification check failed in gateway');
              span.setStatus({ code: api.SpanStatusCode.ERROR, message: 'Invalid credentials' });
              throw Boom.unauthorized('Auth token validation check failed downstream');
            }

            // Trigger checkout on order-service
            const checkoutPayload = {
              user_id: authData.user_id || 'guest_user',
              ...request.payload
            };

            logger.info(`Forwarding checkout order payload to order-service: ${JSON.stringify(checkoutPayload)}`);
            const { status: orderStatus, data: orderData } = await fetchJson(`${ORDER_SERVICE_URL}/orders`, {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify(checkoutPayload)
            });

            span.setAttribute('order.status', orderData.status || 'failed');
            span.setAttribute('order.id', orderData.order_id || 'none');

            // Record custom business metric on checkout success
            if (orderStatus === 200 && orderData.total) {
              checkoutRevenueCounter.add(orderData.total, { tier: tier });
            }

            const traceId = span.spanContext().traceId;
            return h.response({ ...orderData, trace_id: traceId }).code(orderStatus);
          } catch (err) {
            span.recordException(err);
            span.setStatus({ code: api.SpanStatusCode.ERROR, message: err.message });
            logger.error('Failed complete checkout cascade chain execution', { error: err.message });
            if (Boom.isBoom(err)) throw err;
            throw Boom.internal('Checkout cascading failure');
          } finally {
            span.end();
          }
        });
      });
    }
  });

  server.route({
    method: 'GET',
    path: '/{param*}',
    options: { plugins: { skipAuth: true } },
    handler: {
      directory: {
        path: require('path').join(__dirname, 'public'),
        redirectToSlash: true,
        index: true
      }
    }
  });

  await server.start();
  logger.info(`E-Commerce Gateway listening on port ${PORT}`);
};

process.on('unhandledRejection', (err) => {
  console.error(err);
  process.exit(1);
});

init();
