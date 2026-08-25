const api = require('@opentelemetry/api');
const enableObsEnv = (process.env.ENABLE_OBSERVABILITY || 'true').toLowerCase();
const isObservabilityEnabled = !['false', '0', 'no', 'off', 'disable', 'disabled'].includes(enableObsEnv);

if (isObservabilityEnabled) {
  const { NodeSDK } = require('@opentelemetry/sdk-node');
  const { getNodeAutoInstrumentations } = require('@opentelemetry/auto-instrumentations-node');
  const { OTLPTraceExporter } = require('@opentelemetry/exporter-trace-otlp-http');
  const { OTLPMetricExporter } = require('@opentelemetry/exporter-metrics-otlp-http');
  const { OTLPLogExporter } = require('@opentelemetry/exporter-logs-otlp-http');
  const { PeriodicExportingMetricReader } = require('@opentelemetry/sdk-metrics');
  const { BatchLogRecordProcessor, LoggerProvider } = require('@opentelemetry/sdk-logs');
  const { logs } = require('@opentelemetry/api-logs');
  const { W3CTraceContextPropagator } = require('@opentelemetry/core');

  const otlpEndpoint = process.env.OTEL_EXPORTER_OTLP_ENDPOINT || 'http://otel-collector:4318';
  const otlpLogsEndpoint = `${otlpEndpoint}/v1/logs`;
  const otlpTracesEndpoint = `${otlpEndpoint}/v1/traces`;
  const otlpMetricsEndpoint = `${otlpEndpoint}/v1/metrics`;

  const loggerProvider = new LoggerProvider({
    processors: [
      new BatchLogRecordProcessor(new OTLPLogExporter({ url: otlpLogsEndpoint }))
    ]
  });
  logs.setGlobalLoggerProvider(loggerProvider);

  const sdk = new NodeSDK({
    serviceName: process.env.OTEL_SERVICE_NAME || 'node-app',
    textMapPropagator: new W3CTraceContextPropagator(),
    traceExporter: new OTLPTraceExporter({ url: otlpTracesEndpoint }),
    metricReader: new PeriodicExportingMetricReader({
      exporter: new OTLPMetricExporter({ url: otlpMetricsEndpoint }),
      exportIntervalMillis: 10000,
    }),
    instrumentations: [
      getNodeAutoInstrumentations({
        '@opentelemetry/instrumentation-winston': {
          enabled: true,
          disableLogSending: true,
        },
      }),
    ],
  });

  sdk.start();
  console.log('[*] OpenTelemetry SDK initialized successfully');
}

function getActiveExemplar() {
  if (!isObservabilityEnabled) return undefined;
  const activeSpan = api.trace.getActiveSpan();
  if (activeSpan) {
    const spanContext = activeSpan.spanContext();
    if (spanContext && (spanContext.traceFlags & api.TraceFlags.SAMPLED)) {
      return {
        trace_id: spanContext.traceId,
        span_id: spanContext.spanId
      };
    }
  }
  return undefined;
}

const BAGGAGE_KEYS = ['customer.tier', 'correlation.id'];

function setBaggageValue(key, value) {
  if (!isObservabilityEnabled) return api.context.active();
  const activeCtx = api.context.active();
  let baggage = api.propagation.getBaggage(activeCtx) || api.propagation.createBaggage();
  baggage = baggage.setEntry(key, { value: value });
  return api.propagation.setBaggage(activeCtx, baggage);
}

function getBaggageValue(key) {
  if (!isObservabilityEnabled) return undefined;
  const baggage = api.propagation.getBaggage(api.context.active());
  return baggage?.getEntry(key)?.value;
}

function attachBaggageToSpan(span) {
  if (!isObservabilityEnabled || !span) return;
  const baggage = api.propagation.getBaggage(api.context.active());
  if (!baggage) return;
  for (const key of BAGGAGE_KEYS) {
    const val = baggage.getEntry(key)?.value;
    if (val !== undefined) {
      span.setAttribute(key, val);
    }
  }
}

module.exports = {
  api,
  getActiveExemplar,
  isObservabilityEnabled,
  setBaggageValue,
  getBaggageValue,
  attachBaggageToSpan
};
