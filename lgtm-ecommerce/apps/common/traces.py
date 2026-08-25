import os
from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.sdk.resources import Resource
from opentelemetry.trace.propagation.tracecontext import TraceContextTextMapPropagator
from opentelemetry.propagate import set_global_textmap

enable_obs_env = os.getenv("ENABLE_OBSERVABILITY", "true").lower()
is_obs_enabled = enable_obs_env not in ["false", "0", "no", "off", "disable", "disabled"]
service_name = os.getenv("OTEL_SERVICE_NAME", "python-app")

if is_obs_enabled:
    set_global_textmap(TraceContextTextMapPropagator())
    otlp_endpoint = os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://otel-collector:4318")
    resource = Resource.create(attributes={"service.name": service_name})
    trace_provider = TracerProvider(resource=resource)
    trace.set_tracer_provider(trace_provider)
    trace_exporter = OTLPSpanExporter(endpoint=f"{otlp_endpoint}/v1/traces")
    trace_provider.add_span_processor(BatchSpanProcessor(trace_exporter))
    tracer = trace.get_tracer(f"{service_name}-tracer")
else:
    class NoOpTracer:
        def start_as_current_span(self, name, context=None):
            from contextlib import contextmanager
            @contextmanager
            def _noop_span():
                yield None
            return _noop_span()
    tracer = NoOpTracer()

def setup_observability(app):
    if not is_obs_enabled:
        return
    if app.__class__.__name__ == "FastAPI":
        from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
        FastAPIInstrumentor().instrument_app(app)
    else:
        from opentelemetry.instrumentation.flask import FlaskInstrumentor
        FlaskInstrumentor().instrument_app(app)
    try:
        from opentelemetry.instrumentation.psycopg2 import Psycopg2Instrumentor
        Psycopg2Instrumentor().instrument()
    except Exception:
        pass
    try:
        from opentelemetry.instrumentation.kafka import KafkaInstrumentor
        KafkaInstrumentor().instrument()
    except Exception:
        pass

BAGGAGE_KEYS = ["customer.tier", "correlation.id"]

def set_baggage_value(key, val):
    if not is_obs_enabled:
        return
    from opentelemetry import baggage
    from opentelemetry.context import attach
    ctx = baggage.set_baggage(key, val)
    attach(ctx)

def get_baggage_value(key):
    if not is_obs_enabled:
        return None
    from opentelemetry import baggage
    return baggage.get_baggage(key)

def attach_baggage_to_span(span):
    if not is_obs_enabled or not span:
        return
    from opentelemetry import baggage
    for key in BAGGAGE_KEYS:
        val = baggage.get_baggage(key)
        if val is not None:
            span.set_attribute(key, val)

