const { api } = require('./traces');

const serviceName = process.env.OTEL_SERVICE_NAME || 'node-app';
const meter = api.metrics.getMeter(serviceName);

const calculationCounter = meter.createCounter('calculation_requests_total', {
  description: 'Total number of calculation requests processed by Node.js'
});

const errorCounter = meter.createCounter('error_requests_total', {
  description: 'Total number of errored requests in Node.js'
});

const taskDurationHistogram = meter.createHistogram('node_task_duration_seconds', {
  description: 'Duration of nested tasks executed in Node.js',
  unit: 's'
});

const activeUsersGauge = meter.createObservableGauge('node_active_users', {
  description: 'Simulated count of active users on the system'
});

// Auto-observe simulated concurrency load
activeUsersGauge.addCallback((observableResult) => {
  observableResult.observe(Math.floor(Math.random() * 30) + 70);
});

const checkoutRevenueCounter = meter.createCounter('checkout_revenue_total', {
  description: 'Total revenue compiled from checkout checkouts'
});

module.exports = {
  meter,
  calculationCounter,
  errorCounter,
  taskDurationHistogram,
  activeUsersGauge,
  checkoutRevenueCounter
};
