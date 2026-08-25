const http = require('http');
const winston = require('winston');
const Transport = require('winston-transport');
const { api, isObservabilityEnabled } = require('./traces');

class OTelLogTransport extends Transport {
  constructor(opts) {
    super(opts);
    const otlpEndpoint = process.env.OTEL_EXPORTER_OTLP_ENDPOINT || 'http://otel-collector:4318';
    this.url = (opts && opts.url) || `${otlpEndpoint}/v1/logs`;
  }

  log(info, callback) {
    setImmediate(() => {
      this.emit('logged', info);
    });

    const activeSpan = api.trace.getActiveSpan();
    let traceId = '';
    let spanId = '';
    if (activeSpan) {
      const spanContext = activeSpan.spanContext();
      traceId = spanContext.traceId;
      spanId = spanContext.spanId;
    }

    const payload = {
      resourceLogs: [{
        resource: {
          attributes: [
            { key: 'service.name', value: { stringValue: process.env.OTEL_SERVICE_NAME || 'node-app' } }
          ]
        },
        scopeLogs: [{
          scope: { name: 'winston-otel-transport' },
          logRecords: [{
            timeUnixNano: String(Date.now() * 1000000),
            body: { stringValue: info.message },
            severityText: info.level.toUpperCase(),
            severityNumber: info.level === 'error' ? 17 : 9,
            traceId: traceId,
            spanId: spanId
          }]
        }]
      }]
    };

    const data = JSON.stringify(payload);
    const parsedUrl = new URL(this.url);
    const req = http.request({
      hostname: parsedUrl.hostname,
      port: parsedUrl.port,
      path: parsedUrl.pathname,
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Content-Length': data.length
      }
    }, (res) => {
      if (res.statusCode >= 400) {
        console.error('OTelLogTransport response status:', res.statusCode);
      }
      res.resume();
    });

    req.on('error', (err) => {
      console.error('OTelLogTransport error:', err.message);
    });
    req.write(data);
    req.end();

    callback();
  }
}

const winstonFormatWithTrace = winston.format((info) => {
  const span = api.trace.getActiveSpan();
  if (span) {
    const spanContext = span.spanContext();
    if (spanContext && api.trace.isSpanContextValid(spanContext)) {
      info.trace_id = spanContext.traceId;
      info.span_id = spanContext.spanId;
    }
  }
  return info;
});

const transports = [new winston.transports.Console()];
if (isObservabilityEnabled) {
  transports.push(new OTelLogTransport());
}

const logger = winston.createLogger({
  level: 'info',
  format: winston.format.combine(
    winstonFormatWithTrace(),
    winston.format.json()
  ),
  transports
});

module.exports = logger;
